package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/roqem/konen/internal/ui"
)

const (
	defaultKonenRepository  = "roqem/konen"
	defaultMiseRepository   = "jdx/mise"
	maxReleaseMetadataBytes = 4 << 20
	maxChecksumsBytes       = 1 << 20
	maxArchiveBytes         = 100 << 20
	maxExecutableBytes      = 64 << 20
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type publishedRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type updateComponent struct {
	key       string
	name      string
	current   string
	target    string
	action    string
	path      string
	automatic bool
}

type updatePlan struct {
	components []updateComponent
}

func defaultHTTPClient() httpDoer {
	return &http.Client{Timeout: 30 * time.Second}
}

func (a *App) runUpdate(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	dryRun := flags.Bool("dry-run", false, "mostra versões e ações sem atualizar")
	yes := flags.Bool("yes", false, "atualiza sem pedir confirmação")
	pre := flags.Bool("pre", false, "inclui prereleases do Konen")
	var only commaListFlag
	flags.Var(&only, "only", "limita a konen ou mise")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("update não aceita argumentos posicionais")
	}
	if len(only) == 0 {
		only = []string{"konen", "mise"}
	}
	if err := validateUpdateComponents(only); err != nil {
		return err
	}

	plan, err := a.buildUpdatePlan(ctx, only, *pre)
	if err != nil {
		return err
	}
	fmt.Fprint(a.options.Out, renderUpdatePlan(plan))
	if *dryRun {
		fmt.Fprintln(a.options.Out, "Nenhuma atualização foi executada.")
		return nil
	}

	automatic := plan.automaticComponents()
	if len(automatic) == 0 {
		if plan.allCurrent() {
			fmt.Fprintln(a.options.Out, "Os componentes selecionados já estão nas versões planejadas.")
		} else {
			fmt.Fprintln(a.options.Out, "Nenhuma atualização automática está disponível; siga a orientação do plano.")
		}
		return nil
	}
	if !*yes {
		if !a.options.Interactive {
			return errors.New("revise com `konen update --dry-run` ou confirme com `konen update --yes`")
		}
		confirmed, err := a.options.Prompter.Confirm("Aplicar estas atualizações?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(a.options.Out, "Atualização cancelada.")
			return nil
		}
	}

	var stagedKonen string
	if component, ok := plan.component("konen"); ok && component.automatic {
		stagedKonen, err = a.stageKonenUpdate(ctx, component)
		if err != nil {
			return err
		}
		defer os.Remove(stagedKonen)
	}

	if component, ok := plan.component("mise"); ok && component.automatic {
		if err := a.applyMiseUpdate(ctx, component); err != nil {
			return err
		}
		fmt.Fprintf(a.options.Out, "mise atualizado: %s\n", component.target)
	}
	if component, ok := plan.component("konen"); ok && component.automatic {
		if err := installStagedExecutable(stagedKonen, component.path); err != nil {
			return fmt.Errorf("não foi possível substituir o Konen: %w", err)
		}
		stagedKonen = ""
		fmt.Fprintf(a.options.Out, "Konen atualizado: %s\n", component.target)
	}
	return nil
}

func validateUpdateComponents(components []string) error {
	var invalid []string
	for _, component := range components {
		if component != "konen" && component != "mise" {
			invalid = append(invalid, component)
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	return fmt.Errorf("componente desconhecido: %s (use konen ou mise)", strings.Join(invalid, ", "))
}

func (a *App) buildUpdatePlan(ctx context.Context, selected []string, includePrereleases bool) (updatePlan, error) {
	var plan updatePlan
	if containsString(selected, "konen") {
		component, err := a.planKonenUpdate(ctx, includePrereleases)
		if err != nil {
			return updatePlan{}, err
		}
		plan.components = append(plan.components, component)
	}
	if containsString(selected, "mise") {
		component, err := a.planMiseUpdate(ctx)
		if err != nil {
			return updatePlan{}, err
		}
		plan.components = append(plan.components, component)
	}
	return plan, nil
}

func (a *App) planKonenUpdate(ctx context.Context, includePrereleases bool) (updateComponent, error) {
	current := normalizeReleaseVersion(a.options.Version)
	developmentBuild := current == "" || current == "dev"
	allowPrerelease := includePrereleases || developmentBuild || strings.Contains(current, "-")
	target, err := a.latestPublishedVersion(ctx, a.konenReleaseAPIURL(), allowPrerelease)
	if err != nil {
		return updateComponent{}, fmt.Errorf("não foi possível consultar a versão mais recente do Konen: %w", err)
	}
	component := updateComponent{
		key: "konen", name: "Konen", current: displayVersion(current), target: target,
		path: a.konenExecutablePath(),
	}
	switch {
	case current == target:
		component.action = "já está atual"
	case developmentBuild:
		component.action = "instale uma release; builds de desenvolvimento não se autoatualizam"
	case !managedKonenExecutable(component.path, a.options.HomeDir):
		component.action = "atualize pelo gerenciador responsável por " + displayPath(component.path, a.options.HomeDir)
	default:
		component.action = "baixar, verificar checksum e substituir " + displayPath(component.path, a.options.HomeDir)
		component.automatic = true
	}
	return component, nil
}

func (a *App) planMiseUpdate(ctx context.Context) (updateComponent, error) {
	target, err := a.latestPublishedVersion(ctx, a.miseReleaseAPIURL(), false)
	if err != nil {
		return updateComponent{}, fmt.Errorf("não foi possível consultar a versão mais recente do mise: %w", err)
	}
	component := updateComponent{key: "mise", name: "mise", current: "não instalado", target: target}
	path, err := a.findCommand("mise")
	if err != nil {
		component.action = "instale novamente com install.sh"
		return component, nil
	}
	component.path = path
	output, err := a.options.Runner.Output(ctx, "", path, "--version")
	if err != nil {
		return updateComponent{}, fmt.Errorf("não foi possível consultar a versão do mise em %s: %w", path, err)
	}
	current, err := extractVersion(output)
	if err != nil {
		return updateComponent{}, fmt.Errorf("versão do mise não reconhecida em %s: %s", path, strings.TrimSpace(output))
	}
	component.current = current
	switch {
	case current == target:
		component.action = "já está atual"
	case !managedMiseExecutable(path, a.options.BinDir, a.options.HomeDir):
		component.action = "atualize pelo gerenciador responsável por " + displayPath(path, a.options.HomeDir)
	default:
		component.action = "mise self-update " + target + " --yes --no-plugins"
		component.automatic = true
	}
	return component, nil
}

func (a *App) latestPublishedVersion(ctx context.Context, endpoint string, includePrereleases bool) (string, error) {
	if !includePrereleases {
		return a.latestStableVersion(ctx, endpoint)
	}
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := requestURL.Query()
	if query.Get("per_page") == "" {
		query.Set("per_page", "20")
	}
	requestURL.RawQuery = query.Encode()
	data, err := a.download(ctx, requestURL.String(), maxReleaseMetadataBytes)
	if err != nil {
		return "", err
	}
	var releases []publishedRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return "", fmt.Errorf("resposta de releases inválida: %w", err)
	}
	for _, release := range releases {
		if release.Draft {
			continue
		}
		version := normalizeReleaseVersion(release.TagName)
		if validReleaseVersion(version) {
			return version, nil
		}
	}
	return "", errors.New("nenhuma release publicada foi encontrada")
}

func (a *App) latestStableVersion(ctx context.Context, endpoint string) (string, error) {
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/latest") {
		endpoint += "/latest"
	}
	data, err := a.download(ctx, endpoint, maxReleaseMetadataBytes)
	if err != nil {
		return "", err
	}
	var release publishedRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return "", fmt.Errorf("resposta da release estável inválida: %w", err)
	}
	version := normalizeReleaseVersion(release.TagName)
	if release.Draft || release.Prerelease || !validReleaseVersion(version) {
		return "", errors.New("nenhuma release estável foi encontrada")
	}
	return version, nil
}

func (a *App) stageKonenUpdate(ctx context.Context, component updateComponent) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("autoatualização ainda não suporta %s", runtime.GOOS)
	}
	architecture := runtime.GOARCH
	if architecture != "amd64" && architecture != "arm64" {
		return "", fmt.Errorf("arquitetura não suportada: %s", architecture)
	}
	asset := fmt.Sprintf("konen_%s_linux_%s.tar.gz", component.target, architecture)
	downloadRoot := strings.TrimRight(a.konenReleaseBaseURL(), "/") + "/download/v" + component.target
	archive, err := a.download(ctx, downloadRoot+"/"+asset, maxArchiveBytes)
	if err != nil {
		return "", fmt.Errorf("não foi possível baixar %s: %w", asset, err)
	}
	checksums, err := a.download(ctx, downloadRoot+"/checksums.txt", maxChecksumsBytes)
	if err != nil {
		return "", fmt.Errorf("não foi possível baixar checksums.txt: %w", err)
	}
	if err := verifyReleaseChecksum(archive, checksums, asset); err != nil {
		return "", err
	}
	binary, err := executableFromArchive(archive)
	if err != nil {
		return "", err
	}
	staged, err := writeStagedExecutable(component.path, binary)
	if err != nil {
		return "", err
	}
	output, err := a.options.Runner.Output(ctx, "", staged, "version")
	if err != nil || normalizeReleaseVersion(strings.TrimSpace(output)) != component.target {
		os.Remove(staged)
		if err != nil {
			return "", fmt.Errorf("o executável baixado não pôde ser validado: %w", err)
		}
		return "", fmt.Errorf("o executável baixado informou uma versão inesperada: %s", strings.TrimSpace(output))
	}
	return staged, nil
}

func (a *App) applyMiseUpdate(ctx context.Context, component updateComponent) error {
	if err := a.options.Runner.Run(
		ctx, "", component.path, "self-update", component.target, "--yes", "--no-plugins",
	); err != nil {
		return fmt.Errorf("mise self-update: %w", err)
	}
	output, err := a.options.Runner.Output(ctx, "", component.path, "--version")
	if err != nil {
		return fmt.Errorf("mise foi atualizado, mas a versão não pôde ser confirmada: %w", err)
	}
	version, err := extractVersion(output)
	if err != nil || version != component.target {
		return fmt.Errorf("mise informou %q depois de atualizar; esperado %s", strings.TrimSpace(output), component.target)
	}
	return nil
}

func (a *App) download(ctx context.Context, source string, maximum int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "konen/"+displayVersion(normalizeReleaseVersion(a.options.Version)))
	response, err := a.options.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("resposta excede o limite de %d bytes", maximum)
	}
	return data, nil
}

func verifyReleaseChecksum(archive, checksums []byte, asset string) error {
	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./")
		if name == asset {
			expected = fields[0]
			break
		}
	}
	decoded, err := hex.DecodeString(expected)
	if expected == "" || err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("checksum não encontrado ou inválido para %s", asset)
	}
	actual := sha256.Sum256(archive)
	if !bytes.Equal(actual[:], decoded) {
		return fmt.Errorf("checksum inválido para %s", asset)
	}
	return nil
}

func executableFromArchive(archive []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("archive do Konen inválido: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var executable []byte
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("archive do Konen inválido: %w", err)
		}
		if header.Name != "konen" {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size < 1 || header.Size > maxExecutableBytes {
			return nil, errors.New("o archive não contém um executável Konen regular válido")
		}
		if executable != nil {
			return nil, errors.New("o archive contém mais de um executável Konen")
		}
		executable, err = io.ReadAll(io.LimitReader(tarReader, maxExecutableBytes+1))
		if err != nil || int64(len(executable)) != header.Size {
			return nil, errors.New("o executável Konen no archive está truncado")
		}
	}
	if executable == nil {
		return nil, errors.New("o archive não contém o executável konen")
	}
	return executable, nil
}

func writeStagedExecutable(destination string, contents []byte) (string, error) {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".konen-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("não foi possível preparar a atualização ao lado de %s: %w", destination, err)
	}
	path := temporary.Name()
	failed := true
	defer func() {
		if failed {
			temporary.Close()
			os.Remove(path)
		}
	}()
	if err := temporary.Chmod(0o755); err != nil {
		return "", err
	}
	if _, err := temporary.Write(contents); err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	failed = false
	return path, nil
}

func installStagedExecutable(staged, destination string) error {
	if staged == "" {
		return errors.New("executável preparado não encontrado")
	}
	if err := os.Rename(staged, destination); err != nil {
		return err
	}
	if directory, err := os.Open(filepath.Dir(destination)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func managedKonenExecutable(path, home string) bool {
	return regularNonSymlink(path) && pathWithin(path, home)
}

func managedMiseExecutable(path, binDir, home string) bool {
	return regularNonSymlink(path) && pathWithin(path, home) && samePath(path, filepath.Join(binDir, "mise"))
}

func regularNonSymlink(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func pathWithin(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absolutePath)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftPath) == filepath.Clean(rightPath)
}

func (a *App) konenExecutablePath() string {
	if a.options.ExecutablePath != "" {
		return a.options.ExecutablePath
	}
	if a.options.BinDir != "" {
		return filepath.Join(a.options.BinDir, "konen")
	}
	return ""
}

func (a *App) konenReleaseAPIURL() string {
	if value := strings.TrimSpace(a.options.Getenv("KONEN_RELEASE_API_URL")); value != "" {
		return value
	}
	repository := strings.TrimSpace(a.options.Getenv("KONEN_REPOSITORY"))
	if repository == "" {
		repository = defaultKonenRepository
	}
	return "https://api.github.com/repos/" + repository + "/releases"
}

func (a *App) miseReleaseAPIURL() string {
	if value := strings.TrimSpace(a.options.Getenv("KONEN_MISE_RELEASE_API_URL")); value != "" {
		return value
	}
	return "https://api.github.com/repos/" + defaultMiseRepository + "/releases"
}

func (a *App) konenReleaseBaseURL() string {
	if value := strings.TrimSpace(a.options.Getenv("KONEN_RELEASE_BASE_URL")); value != "" {
		return value
	}
	repository := strings.TrimSpace(a.options.Getenv("KONEN_REPOSITORY"))
	if repository == "" {
		repository = defaultKonenRepository
	}
	return "https://github.com/" + repository + "/releases"
}

func normalizeReleaseVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func validReleaseVersion(version string) bool {
	if version == "" {
		return false
	}
	for _, character := range version {
		if character >= '0' && character <= '9' ||
			character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' ||
			strings.ContainsRune("._-", character) {
			continue
		}
		return false
	}
	return true
}

func displayVersion(version string) string {
	if version == "" {
		return "desconhecida"
	}
	return version
}

func displayPath(path, home string) string {
	if path == "" {
		return "caminho desconhecido"
	}
	if home != "" {
		if relative, err := filepath.Rel(home, path); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			if relative == "." {
				return "~"
			}
			return "~/" + filepath.ToSlash(relative)
		}
	}
	return path
}

func renderUpdatePlan(plan updatePlan) string {
	rows := make([][]string, 0, len(plan.components))
	for _, component := range plan.components {
		rows = append(rows, []string{component.name, component.current, component.target, component.action})
	}
	return ui.RenderTable([]string{"Componente", "Atual", "Disponível", "Plano"}, rows)
}

func (p updatePlan) automaticComponents() []updateComponent {
	var components []updateComponent
	for _, component := range p.components {
		if component.automatic {
			components = append(components, component)
		}
	}
	return components
}

func (p updatePlan) component(key string) (updateComponent, bool) {
	for _, component := range p.components {
		if component.key == key {
			return component, true
		}
	}
	return updateComponent{}, false
}

func (p updatePlan) allCurrent() bool {
	if len(p.components) == 0 {
		return false
	}
	for _, component := range p.components {
		if component.current != component.target {
			return false
		}
	}
	return true
}
