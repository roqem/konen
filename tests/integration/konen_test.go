//go:build linux

package integration_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const testMiseVersion = "2026.8.15"

func TestInstallerInstallsUpdatesAndRejectsCorruption(t *testing.T) {
	architecture, miseArchitecture := releaseArchitectures(t)
	root := t.TempDir()
	home := filepath.Join(root, "home")
	tempDir := filepath.Join(root, "tmp")
	releases := filepath.Join(root, "releases")
	for _, path := range []string{home, tempDir, releases} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	latest := &latestRelease{}
	server := newReleaseServer(t, releases, latest)
	defer server.Close()
	createMiseRelease(t, releases, miseArchitecture)

	firstVersion := "0.1.0-test.1"
	createKonenRelease(t, releases, firstVersion, architecture)
	latest.Set(firstVersion)

	sudoMarker := filepath.Join(root, "sudo-was-called")
	shimDir := filepath.Join(root, "shims")
	for _, command := range []string{"awk", "curl", "gzip", "install", "mkdir", "mktemp", "rm", "sha256sum", "tar", "uname"} {
		linkCommand(t, shimDir, command)
	}
	writeExecutable(t, filepath.Join(shimDir, "sudo"), "#!/bin/sh\n: > \"$KONEN_TEST_SUDO_MARKER\"\nexit 99\n")

	environment := testEnvironment(map[string]string{
		"HOME":                        home,
		"PATH":                        shimDir,
		"SHELL":                       "/bin/sh",
		"TMPDIR":                      tempDir,
		"KONEN_RELEASE_BASE_URL":      server.URL + "/konen",
		"KONEN_MISE_RELEASE_BASE_URL": server.URL + "/mise",
		"KONEN_MISE_VERSION":          testMiseVersion,
		"KONEN_INSTALL_DIR":           "",
		"KONEN_INSTALL_MISE":          "1",
		"KONEN_VERSION":               "",
		"KONEN_TEST_SUDO_MARKER":      sudoMarker,
	})

	repository := repositoryRoot(t)
	installScript := filepath.Join(repository, "install.sh")
	output := runCommand(t, repository, environment, "/bin/sh", installScript)
	assertContains(t, output, "Konen "+firstVersion+" instalado")
	assertContains(t, output, "mise "+testMiseVersion+" instalado")

	installedKonen := filepath.Join(home, ".local", "bin", "konen")
	installedMise := filepath.Join(home, ".local", "bin", "mise")
	assertExecutable(t, installedKonen)
	assertExecutable(t, installedMise)
	if got := strings.TrimSpace(runCommand(t, root, environment, installedKonen, "version")); got != firstVersion {
		t.Fatalf("installed version = %q, want %q", got, firstVersion)
	}
	if got := strings.TrimSpace(runCommand(t, root, environment, installedMise, "--version")); !strings.HasPrefix(got, testMiseVersion) {
		t.Fatalf("mise version = %q, want prefix %q", got, testMiseVersion)
	}
	if _, err := os.Stat(sudoMarker); !os.IsNotExist(err) {
		t.Fatalf("installer invoked sudo; marker error = %v", err)
	}

	stateDir := filepath.Join(home, "personal-state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateMarker := filepath.Join(stateDir, "keep-me")
	if err := os.WriteFile(stateMarker, []byte("preserved\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	secondVersion := "0.1.0-test.2"
	secondArchive := createKonenRelease(t, releases, secondVersion, architecture)
	latest.Set(secondVersion)
	output = runCommand(t, repository, environment, "/bin/sh", installScript)
	assertContains(t, output, "Konen "+secondVersion+" instalado")
	if got := strings.TrimSpace(runCommand(t, root, environment, installedKonen, "version")); got != secondVersion {
		t.Fatalf("updated version = %q, want %q", got, secondVersion)
	}
	if data, err := os.ReadFile(stateMarker); err != nil || string(data) != "preserved\n" {
		t.Fatalf("state was not preserved across update: data=%q error=%v", data, err)
	}

	installedDigest := fileDigest(t, installedKonen)
	archive, err := os.OpenFile(secondArchive, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := archive.WriteString("corruption"); err != nil {
		archive.Close()
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	failedOutput, err := runCommandError(repository, environment, "/bin/sh", installScript)
	if err == nil {
		t.Fatal("installer accepted a corrupted release archive")
	}
	assertContains(t, failedOutput, "checksum inválido")
	if got := fileDigest(t, installedKonen); got != installedDigest {
		t.Fatal("failed update replaced the working Konen executable")
	}
	if data, err := os.ReadFile(stateMarker); err != nil || string(data) != "preserved\n" {
		t.Fatalf("failed update changed state: data=%q error=%v", data, err)
	}
}

func TestFirstRunJourneyThroughBuiltExecutable(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	binDir := filepath.Join(root, "bin")
	configDir := filepath.Join(root, "config")
	tempDir := filepath.Join(root, "tmp")
	stateDir := filepath.Join(home, "machine-state")
	projectDir := filepath.Join(home, "Projects", "sample")
	for _, path := range []string{home, binDir, configDir, tempDir, projectDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	konen := filepath.Join(binDir, "konen")
	buildKonen(t, konen, "journey-test")
	miseLog := filepath.Join(root, "mise.log")
	applyMarker := filepath.Join(root, "mise-applied")
	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$KONEN_TEST_MISE_LOG"
printf 'state-env=%s|%s|%s|%s\n' \
  "${MISE_GLOBAL_CONFIG_FILE:-}" \
  "${MISE_GLOBAL_CONFIG_ROOT:-}" \
  "${MISE_CEILING_PATHS:-}" \
  "${MISE_OVERRIDE_CONFIG_FILENAMES:-}" >> "$KONEN_TEST_MISE_LOG"
if [ "${1:-}" = "--version" ]; then
  printf '2026.8.15 linux-x64 (integration fixture)\n'
  exit 0
fi
case " $* " in
  *" bootstrap status --json "*)
    if [ -f "$KONEN_TEST_APPLY_MARKER" ]; then
      printf '%s\n' '{"tools":[{"tool":"go","requested_version":"1.27.0","resolved_version":"1.27.0","state":"installed","installed":true}]}'
    else
      printf '%s\n' '{"tools":[{"tool":"go","requested_version":"1.27.0","resolved_version":"1.27.0","state":"missing","installed":false}]}'
    fi
    ;;
  *" bootstrap --dry-run "*|*" bootstrap --only task --dry-run "*)
    printf 'fixture apply: dry run\n'
    ;;
  *" bootstrap --yes "*)
    if [ -f "$KONEN_TEST_APPLY_MARKER" ]; then
      printf 'fixture apply: go already ready\n'
    else
      : > "$KONEN_TEST_APPLY_MARKER"
      printf 'fixture apply: changed go\n'
    fi
    ;;
  " trust "*)
    ;;
  *)
    printf 'unexpected mise invocation: %s\n' "$*" >&2
    exit 64
    ;;
esac
`)

	environment := testEnvironment(map[string]string{
		"HOME":                    home,
		"XDG_CONFIG_HOME":         configDir,
		"PATH":                    binDir + ":/usr/bin:/bin",
		"SHELL":                   "/bin/sh",
		"TMPDIR":                  tempDir,
		"KONEN_TEST_MISE_LOG":     miseLog,
		"KONEN_TEST_APPLY_MARKER": applyMarker,
	})

	output := runCommand(t, root, environment, konen, "init", "--git", stateDir)
	assertContains(t, output, "Konen configurado")
	assertContains(t, output, "O Konen não cria commits")
	assertContains(t, output, "Próximo passo: execute `konen apply`.")
	for _, expected := range []string{
		filepath.Join(stateDir, ".git"),
		filepath.Join(stateDir, "mise.toml"),
		filepath.Join(stateDir, "home"),
		filepath.Join(stateDir, "projects"),
		filepath.Join(configDir, "konen", "config.toml"),
	} {
		if _, err := os.Stat(expected); err != nil {
			t.Fatalf("first run did not create %s: %v", expected, err)
		}
	}

	output = runCommand(t, root, environment, konen, "doctor")
	assertContains(t, output, "✓ configuração:")
	assertContains(t, output, "✓ mise: "+testMiseVersion)
	assertContains(t, output, "✓ git:")

	output = runCommand(t, root, environment, konen, "status")
	for _, fragment := range []string{
		"Tipo", "Ferramenta", "go", "1.27.0", "ausente",
		"Backup Git", "Primeiro commit", "pendente", "Remoto", "ausente",
		"git -C", "diff --cached", "gh repo create", "nenhum desses comandos foi executado",
	} {
		assertContains(t, output, fragment)
	}
	output = runCommand(t, root, environment, konen, "status", "--only", "tools", "--state", "missing")
	for _, fragment := range []string{"Ferramenta", "go", "ausente"} {
		assertContains(t, output, fragment)
	}
	if strings.Contains(output, "Backup Git") {
		t.Fatalf("filtered status included backup guidance:\n%s", output)
	}
	output = runCommand(t, root, environment, konen, "status", "--only", "dotfiles", "--state", "pending")
	assertContains(t, output, "Nenhum item corresponde aos filtros.")
	if _, err := runCommandError(stateDir, environment, "git", "rev-parse", "--verify", "HEAD"); err == nil {
		t.Fatal("status created the first Git commit")
	}
	if remotes := strings.TrimSpace(runCommand(t, stateDir, environment, "git", "remote")); remotes != "" {
		t.Fatalf("status created a Git remote: %q", remotes)
	}
	output = runCommand(t, root, environment, konen, "plan")
	assertContains(t, output, "fixture apply: dry run")
	output = runCommand(t, root, environment, konen, "apply", "--dry-run")
	assertContains(t, output, "fixture apply: dry run")
	if _, err := os.Stat(applyMarker); !os.IsNotExist(err) {
		t.Fatalf("dry-run applied the fixture: %v", err)
	}
	output = runCommand(t, root, environment, konen, "apply", "--yes")
	for _, fragment := range []string{
		"fixture apply: changed go", "Resumo do apply", "Convergiram nesta execução",
		"Ferramenta: 1", "Estado declarativo", "convergido",
	} {
		assertContains(t, output, fragment)
	}
	if _, err := os.Stat(applyMarker); err != nil {
		t.Fatalf("real fixture apply did not run: %v", err)
	}
	output = runCommand(t, root, environment, konen, "apply", "--yes")
	for _, fragment := range []string{
		"fixture apply: go already ready", "Convergiram nesta execução", "Já estavam prontos",
		"Ferramenta: 1",
	} {
		assertContains(t, output, fragment)
	}

	installerMarker := filepath.Join(root, "installer-was-run")
	installerSource := filepath.Join(root, "install-noop")
	installerContents := "#!/bin/sh\n#MISE description=\"Integration installer\"\nset -eu\nprintf executed > \"" + installerMarker + "\"\n"
	writeExecutable(t, installerSource, installerContents)
	output = runCommand(t, root, environment, konen,
		"installer", "add", "--dry-run", "--from", installerSource, "noop")
	assertContains(t, output, "Instalador pessoal: noop")
	assertContains(t, output, "Seleção proposta no bootstrap sequencial")
	assertContains(t, output, "Nenhum arquivo foi gravado e nenhuma tarefa foi executada")
	installerDestination := filepath.Join(stateDir, "mise-tasks", "install", "noop")
	if _, err := os.Lstat(installerDestination); !os.IsNotExist(err) {
		t.Fatalf("installer dry run created its destination: %v", err)
	}
	output = runCommand(t, root, environment, konen,
		"installer", "add", "--yes", "--from", installerSource, "noop")
	assertContains(t, output, "O instalador não foi executado durante o cadastro")
	assertExecutable(t, installerDestination)
	if got, err := os.ReadFile(installerDestination); err != nil || string(got) != installerContents {
		t.Fatalf("imported installer = %q, error=%v", got, err)
	}
	if _, err := os.Stat(installerMarker); !os.IsNotExist(err) {
		t.Fatalf("guided add ran the installer: %v", err)
	}
	output = runCommand(t, root, environment, konen, "status")
	assertContains(t, output, "Instalador pessoal")
	assertContains(t, output, "install:noop")
	output = runCommand(t, root, environment, konen, "plan", "--only", "task")
	assertContains(t, output, "fixture apply: dry run")
	if _, err := os.Stat(installerMarker); !os.IsNotExist(err) {
		t.Fatalf("installer plan ran the task: %v", err)
	}

	completion := runCommand(t, root, environment, konen, "completion", "zsh")
	assertContains(t, completion, "#compdef konen")
	assertContains(t, completion, "__complete projects")
	assertContains(t, completion, "dotfile")
	assertContains(t, completion, "installer")
	assertContains(t, completion, "--state")
	assertContains(t, completion, "ready pending missing different unknown")

	manifestPath := filepath.Join(stateDir, "projects", "sample.toml")
	manifest := `version = 1
path = "~/Projects/sample"
keep_invoking_tab = true

[[tabs]]
title = "Terminal"
command = "git status"
hold = true
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	output = runCommand(t, root, environment, konen, "projects")
	assertContains(t, output, "sample")
	assertContains(t, output, "revisão necessária")
	output = runCommand(t, root, environment, konen, "project", "show", "sample")
	assertContains(t, output, `command = "git status"`)
	output = runCommand(t, root, environment, konen, "dev", "sample", "--dry-run")
	assertContains(t, output, "Aprovação local: pendente")
	output = runCommand(t, root, environment, konen, "project", "trust", "sample")
	assertContains(t, output, "Projeto aprovado: sample")
	output = runCommand(t, root, environment, konen, "projects")
	assertContains(t, output, "aprovado")
	output = runCommand(t, root, environment, konen, "sample", "--dry-run")
	assertContains(t, output, "Projeto: sample")
	assertContains(t, output, "Aprovação local: válida")
	output = runCommand(t, projectDir, environment, konen, "dev", "--dry-run")
	assertContains(t, output, "Projeto: sample")
	assertContains(t, output, "Aprovação local: válida")
	if got := runCommand(t, root, environment, konen, "__complete", "projects"); got != "sample\n" {
		t.Fatalf("dynamic completion = %q, want %q", got, "sample\\n")
	}

	logData, err := os.ReadFile(miseLog)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	for _, invocation := range []string{
		"trust " + filepath.Join(stateDir, "mise.toml"),
		"bootstrap status --json",
		"bootstrap --dry-run",
		"bootstrap --yes",
	} {
		assertContains(t, log, invocation)
	}
	assertContains(t, log, "state-env="+filepath.Join(stateDir, "mise.toml")+"|"+stateDir+"|"+stateDir+"|mise.toml")
}

type latestRelease struct {
	mu      sync.RWMutex
	version string
}

func (r *latestRelease) Set(version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.version = version
}

func (r *latestRelease) Get() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

func newReleaseServer(t *testing.T, root string, latest *latestRelease) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/konen/latest" {
			http.Redirect(response, request, "/konen/tag/v"+latest.Get(), http.StatusFound)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/konen/tag/v") {
			response.WriteHeader(http.StatusOK)
			return
		}
		clean := strings.TrimPrefix(filepath.Clean(request.URL.Path), string(filepath.Separator))
		if clean == "." || strings.HasPrefix(clean, "..") {
			http.NotFound(response, request)
			return
		}
		path := filepath.Join(root, clean)
		if relative, err := filepath.Rel(root, path); err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			http.NotFound(response, request)
			return
		}
		http.ServeFile(response, request, path)
	}))
}

func createKonenRelease(t *testing.T, root, version, architecture string) string {
	t.Helper()
	repository := repositoryRoot(t)
	distDir := t.TempDir()
	command := exec.Command(filepath.Join(repository, "scripts", "build-release.sh"), version, distDir)
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build release %s: %v\n%s", version, err, output)
	}

	releaseDir := filepath.Join(root, "konen", "download", "v"+version)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	archiveName := fmt.Sprintf("konen_%s_linux_%s.tar.gz", version, architecture)
	archivePath := filepath.Join(releaseDir, archiveName)
	copyFile(t, filepath.Join(distDir, archiveName), archivePath, 0o644)
	copyFile(t, filepath.Join(distDir, "checksums.txt"), filepath.Join(releaseDir, "checksums.txt"), 0o644)
	return archivePath
}

func createMiseRelease(t *testing.T, root, architecture string) {
	t.Helper()
	releaseDir := filepath.Join(root, "mise", "download", "v"+testMiseVersion)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetName := fmt.Sprintf("mise-v%s-linux-%s", testMiseVersion, architecture)
	assetPath := filepath.Join(releaseDir, assetName)
	writeExecutable(t, assetPath, "#!/bin/sh\nprintf '"+testMiseVersion+" linux-fixture\\n'\n")
	writeChecksums(t, filepath.Join(releaseDir, "SHASUMS256.txt"), assetName, assetPath)
}

func copyFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	sourceFile, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer sourceFile.Close()
	destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		destinationFile.Close()
		t.Fatal(err)
	}
	if err := destinationFile.Close(); err != nil {
		t.Fatal(err)
	}
}

func buildKonen(t *testing.T, destination, version string) {
	t.Helper()
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go is required for the integration harness: %v", err)
	}
	command := exec.Command(goBinary, "build", "-trimpath", "-ldflags=-X main.version="+version, "-o", destination, "./cmd/konen")
	command.Dir = repositoryRoot(t)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build Konen: %v\n%s", err, output)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func releaseArchitectures(t *testing.T) (string, string) {
	t.Helper()
	switch runtime.GOARCH {
	case "amd64":
		return "amd64", "x64"
	case "arm64":
		return "arm64", "arm64"
	default:
		t.Skipf("installer does not support linux/%s", runtime.GOARCH)
		return "", ""
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}

func linkCommand(t *testing.T, directory, name string) {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("integration prerequisite %s is unavailable: %v", name, err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, filepath.Join(directory, name)); err != nil {
		t.Fatal(err)
	}
}

func writeChecksums(t *testing.T, path, name, target string) {
	t.Helper()
	digest := fileDigest(t, target)
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%s  %s\n", digest, name)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		t.Fatalf("%s is not executable: %v", path, info.Mode())
	}
}

func runCommand(t *testing.T, dir string, environment []string, name string, args ...string) string {
	t.Helper()
	output, err := runCommandError(dir, environment, name, args...)
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return output
}

func runCommandError(dir string, environment []string, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = environment
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}

func testEnvironment(overrides map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func assertContains(t *testing.T, value, fragment string) {
	t.Helper()
	if !strings.Contains(value, fragment) {
		t.Fatalf("output does not contain %q:\n%s", fragment, value)
	}
}
