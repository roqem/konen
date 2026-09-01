package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateShowsPlanBeforeDownloadingAndAppliesVerifiedComponents(t *testing.T) {
	const currentKonen = "0.1.0-alpha.1"
	const targetKonen = "0.1.0-alpha.2"
	const currentMise = "2026.8.15"
	const targetMise = "2026.9.1"
	archive := updateArchive(t, []byte("new-konen-binary"))
	asset := "konen_" + targetKonen + "_linux_" + runtimeArchitecture(t) + ".tar.gz"
	digest := sha256.Sum256(archive)
	assetRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/konen-api":
			fmt.Fprintf(response, `[{"tag_name":"v%s","prerelease":true}]`, targetKonen)
		case "/mise-api/latest":
			fmt.Fprintf(response, `{"tag_name":"v%s"}`, targetMise)
		case "/releases/download/v" + targetKonen + "/" + asset:
			assetRequests++
			response.Write(archive)
		case "/releases/download/v" + targetKonen + "/checksums.txt":
			assetRequests++
			fmt.Fprintf(response, "%x  %s\n", digest, asset)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	konenPath := filepath.Join(binDir, "konen")
	misePath := filepath.Join(binDir, "mise")
	if err := os.WriteFile(konenPath, []byte("old-konen-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(misePath, []byte("mise-fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	miseUpdated := false
	runner := &fakeRunner{
		outputHook: func(call runCall) (string, error) {
			switch {
			case call.name == misePath && reflect.DeepEqual(call.args, []string{"--version"}):
				if miseUpdated {
					return targetMise + " linux-fixture", nil
				}
				return currentMise + " linux-fixture", nil
			case strings.HasPrefix(filepath.Base(call.name), ".konen-update-") && reflect.DeepEqual(call.args, []string{"version"}):
				return targetKonen + "\n", nil
			default:
				return "", fmt.Errorf("unexpected output call: %#v", call)
			}
		},
		runHook: func(call runCall) error {
			if call.name != misePath || !reflect.DeepEqual(call.args, []string{
				"self-update", targetMise, "--yes", "--no-plugins",
			}) {
				return fmt.Errorf("unexpected run call: %#v", call)
			}
			miseUpdated = true
			return nil
		},
	}
	var out bytes.Buffer
	application := New(Options{
		HomeDir: home, BinDir: binDir, ExecutablePath: konenPath,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
		Version: currentKonen, HTTPClient: server.Client(), Getenv: updateTestEnvironment(server.URL),
	})

	if err := application.Run(context.Background(), []string{"update", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Componente", "Atual", "Disponível", "Plano",
		"Konen", currentKonen, targetKonen, "baixar, verificar checksum",
		"mise", currentMise, targetMise, "mise self-update",
		"Nenhuma atualização foi executada",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("update dry-run is missing %q:\n%s", fragment, out.String())
		}
	}
	if assetRequests != 0 || miseUpdated {
		t.Fatalf("dry-run mutated state: asset requests=%d mise updated=%t", assetRequests, miseUpdated)
	}
	if got, _ := os.ReadFile(konenPath); string(got) != "old-konen-binary" {
		t.Fatalf("dry-run replaced Konen: %q", got)
	}

	runner.runs = nil
	out.Reset()
	if err := application.Run(context.Background(), []string{"update", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if assetRequests != 2 || !miseUpdated {
		t.Fatalf("update requests=%d mise updated=%t", assetRequests, miseUpdated)
	}
	if got, _ := os.ReadFile(konenPath); string(got) != "new-konen-binary" {
		t.Fatalf("installed Konen = %q", got)
	}
	info, err := os.Stat(konenPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("installed Konen mode = %v", info.Mode())
	}
	for _, fragment := range []string{"mise atualizado: " + targetMise, "Konen atualizado: " + targetKonen} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("update output is missing %q:\n%s", fragment, out.String())
		}
	}
}

func TestUpdateRejectsCorruptionBeforeMutatingEitherComponent(t *testing.T) {
	const targetKonen = "0.1.0-alpha.2"
	archive := updateArchive(t, []byte("corrupted-candidate"))
	asset := "konen_" + targetKonen + "_linux_" + runtimeArchitecture(t) + ".tar.gz"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/konen-api":
			fmt.Fprintf(response, `[{"tag_name":"v%s","prerelease":true}]`, targetKonen)
		case "/mise-api/latest":
			fmt.Fprint(response, `{"tag_name":"v2026.9.1"}`)
		case "/releases/download/v" + targetKonen + "/" + asset:
			response.Write(archive)
		case "/releases/download/v" + targetKonen + "/checksums.txt":
			fmt.Fprintf(response, "%064d  %s\n", 0, asset)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	konenPath := filepath.Join(binDir, "konen")
	misePath := filepath.Join(binDir, "mise")
	for path, contents := range map[string]string{konenPath: "old", misePath: "mise"} {
		if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeRunner{
		outputHook: func(call runCall) (string, error) {
			if call.name == misePath {
				return "2026.8.15 linux-fixture", nil
			}
			return targetKonen, nil
		},
		runHook: func(call runCall) error { return fmt.Errorf("mutation must not run: %#v", call) },
	}
	application := New(Options{
		HomeDir: home, BinDir: binDir, ExecutablePath: konenPath,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
		Version: "0.1.0-alpha.1", HTTPClient: server.Client(), Getenv: updateTestEnvironment(server.URL),
	})

	err := application.Run(context.Background(), []string{"update", "--yes"})
	if err == nil || !strings.Contains(err.Error(), "checksum inválido") {
		t.Fatalf("corrupted update error = %v", err)
	}
	if got, _ := os.ReadFile(konenPath); string(got) != "old" {
		t.Fatalf("corrupted update replaced Konen: %q", got)
	}
	for _, call := range runner.runs {
		if call.name == misePath && len(call.args) > 0 && call.args[0] == "self-update" {
			t.Fatalf("corrupted update mutated mise: %#v", runner.runs)
		}
	}
}

func TestUpdateRequiresConfirmationOnlyWhenItCanMutate(t *testing.T) {
	server := updateMetadataServer(t, "0.1.0-alpha.2", true, "2026.8.15")
	defer server.Close()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	konenPath := filepath.Join(binDir, "konen")
	misePath := filepath.Join(binDir, "mise")
	if err := os.WriteFile(konenPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(misePath, []byte("mise"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	runner := &fakeRunner{outputHook: func(call runCall) (string, error) {
		if call.name == misePath {
			return "2026.8.15 linux-fixture", nil
		}
		return "", fmt.Errorf("unexpected output call: %#v", call)
	}}
	application := New(Options{
		HomeDir: home, BinDir: binDir, ExecutablePath: konenPath,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
		Version: "0.1.0-alpha.1", HTTPClient: server.Client(), Getenv: updateTestEnvironment(server.URL),
	})

	err := application.Run(context.Background(), []string{"update"})
	if err == nil || !strings.Contains(err.Error(), "konen update --dry-run") || !strings.Contains(out.String(), "0.1.0-alpha.2") {
		t.Fatalf("non-interactive update error=%v output=%s", err, out.String())
	}
	if got, _ := os.ReadFile(konenPath); string(got) != "old" {
		t.Fatalf("unconfirmed update replaced Konen: %q", got)
	}
}

func TestUpdateLeavesExternallyManagedMiseToItsOwner(t *testing.T) {
	server := updateMetadataServer(t, "0.1.0", false, "2026.9.1")
	defer server.Close()
	root := t.TempDir()
	externalMise := filepath.Join(root, "external", "mise")
	if err := os.MkdirAll(filepath.Dir(externalMise), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(externalMise, []byte("mise"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	runner := &fakeRunner{
		paths:      map[string]string{"mise": externalMise},
		outputHook: func(call runCall) (string, error) { return "2026.8.15 linux-fixture", nil },
		runHook:    func(call runCall) error { return fmt.Errorf("external mise was mutated: %#v", call) },
	}
	application := New(Options{
		HomeDir: filepath.Join(root, "home"), BinDir: filepath.Join(root, "home", "bin"),
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
		Version: "0.1.0", HTTPClient: server.Client(), Getenv: updateTestEnvironment(server.URL),
	})

	if err := application.Run(context.Background(), []string{"update", "--only", "mise", "--yes"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"atualize pelo gerenciador responsável", externalMise, "Nenhuma atualização automática"} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("external mise plan is missing %q:\n%s", fragment, out.String())
		}
	}
}

func TestUpdateLeavesKonenOutsideHomeToItsPackageManager(t *testing.T) {
	server := updateMetadataServer(t, "1.0.1", false, "2026.8.15")
	defer server.Close()
	root := t.TempDir()
	externalKonen := filepath.Join(root, "opt", "konen")
	if err := os.MkdirAll(filepath.Dir(externalKonen), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(externalKonen, []byte("package-managed"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	application := New(Options{
		HomeDir: filepath.Join(root, "home"), BinDir: filepath.Dir(externalKonen), ExecutablePath: externalKonen,
		Out: &out, Err: &out, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
		Version: "1.0.0", HTTPClient: server.Client(), Getenv: updateTestEnvironment(server.URL),
	})

	if err := application.Run(context.Background(), []string{"update", "--only", "konen", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "atualize pelo gerenciador responsável") {
		t.Fatalf("external Konen plan = %s", out.String())
	}
	if got, _ := os.ReadFile(externalKonen); string(got) != "package-managed" {
		t.Fatalf("external Konen was replaced: %q", got)
	}
}

func TestDevelopmentBuildCanInspectPrereleasesWithoutSelfUpdating(t *testing.T) {
	server := updateMetadataServer(t, "0.1.0-alpha.15", true, "2026.8.15")
	defer server.Close()
	var out bytes.Buffer
	application := New(Options{
		HomeDir: t.TempDir(), Out: &out, Err: &out, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
		Version: "dev", HTTPClient: server.Client(), Getenv: updateTestEnvironment(server.URL),
	})

	if err := application.Run(context.Background(), []string{"update", "--dry-run", "--only", "konen"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"dev", "0.1.0-alpha.15", "builds de desenvolvimento não se autoatualizam"} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("development update plan is missing %q:\n%s", fragment, out.String())
		}
	}
}

func TestUpdateRejectsUnknownComponentsBeforeNetworkAccess(t *testing.T) {
	application := New(Options{
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
		HTTPClient: failingHTTPClient{},
	})
	err := application.Run(context.Background(), []string{"update", "--only", "browser"})
	if err == nil || !strings.Contains(err.Error(), "componente desconhecido") {
		t.Fatalf("unknown update component error = %v", err)
	}
}

func TestManagedExecutableRejectsAParentSymlinkOutsideHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	external := filepath.Join(root, "external")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "konen"), []byte("external"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(home, "bin")); err != nil {
		t.Fatal(err)
	}
	if managedKonenExecutable(filepath.Join(home, "bin", "konen"), home) {
		t.Fatal("an executable reached through a parent symlink outside home was considered managed")
	}
}

func TestLatestPublishedVersionRespectsStableAndPrereleaseChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/latest" {
			fmt.Fprint(response, `{"tag_name":"v1.0.1"}`)
			return
		}
		fmt.Fprint(response, `[
  {"tag_name":"v1.1.0-alpha.2","prerelease":true},
  {"tag_name":"v1.0.1"}
]`)
	}))
	defer server.Close()
	application := New(Options{HTTPClient: server.Client()})

	stable, err := application.latestPublishedVersion(context.Background(), server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	prerelease, err := application.latestPublishedVersion(context.Background(), server.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	if stable != "1.0.1" || prerelease != "1.1.0-alpha.2" {
		t.Fatalf("stable=%q prerelease=%q", stable, prerelease)
	}
}

func updateArchive(t *testing.T, executable []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "konen", Mode: 0o755, Size: int64(len(executable)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(executable); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func updateMetadataServer(t *testing.T, konenVersion string, prerelease bool, miseVersion string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/konen-api":
			fmt.Fprintf(response, `[{"tag_name":"v%s","prerelease":%t}]`, konenVersion, prerelease)
		case "/konen-api/latest":
			fmt.Fprintf(response, `{"tag_name":"v%s","prerelease":%t}`, konenVersion, prerelease)
		case "/mise-api/latest":
			fmt.Fprintf(response, `{"tag_name":"v%s"}`, miseVersion)
		default:
			http.NotFound(response, request)
		}
	}))
}

func updateTestEnvironment(serverURL string) func(string) string {
	return func(key string) string {
		switch key {
		case "KONEN_RELEASE_API_URL":
			return serverURL + "/konen-api"
		case "KONEN_MISE_RELEASE_API_URL":
			return serverURL + "/mise-api"
		case "KONEN_RELEASE_BASE_URL":
			return serverURL + "/releases"
		default:
			return ""
		}
	}
}

func runtimeArchitecture(t *testing.T) string {
	t.Helper()
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skipf("update test does not support %s", runtime.GOARCH)
	}
	return runtime.GOARCH
}

type failingHTTPClient struct{}

func (failingHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected network request")
}
