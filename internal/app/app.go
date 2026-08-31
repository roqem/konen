package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/roqem/konen/internal/config"
	"github.com/roqem/konen/internal/execx"
	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

type Options struct {
	ConfigPath  string
	HomeDir     string
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	Runner      execx.Runner
	Prompter    ui.Prompter
	Interactive bool
	Version     string
	BinDir      string
	WorkDir     string
	Getenv      func(string) string
}

type App struct {
	options Options
	state   state.Service
}

const minimumMiseVersion = "2026.8.15"

func New(options Options) *App {
	if options.Getenv == nil {
		options.Getenv = os.Getenv
	}
	if options.WorkDir == "" {
		options.WorkDir, _ = os.Getwd()
	}
	return &App{options: options, state: state.Service{Runner: options.Runner}}
}

func (a *App) Run(ctx context.Context, args []string) error {
	err := a.run(ctx, args)
	if ui.IsUserAborted(err) {
		return nil
	}
	return err
}

func (a *App) run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		if !a.options.Interactive {
			a.printHelp()
			return errors.New("nenhum comando informado em uma sessão não interativa")
		}
		_, err := a.loadState()
		configured := err == nil
		action, err := a.options.Prompter.Menu(configured)
		if err != nil {
			return err
		}
		return a.run(ctx, []string{action})
	}

	switch args[0] {
	case "init":
		return a.runInit(ctx, args[1:])
	case "status":
		return a.runStatus(ctx, args[1:])
	case "plan":
		return a.runApply(ctx, append([]string{"--dry-run"}, args[1:]...))
	case "diff":
		return a.runMise(ctx, []string{"bootstrap", "dotfiles", "diff"})
	case "apply":
		return a.runApply(ctx, args[1:])
	case "tool":
		return a.runTool(ctx, args[1:])
	case "package":
		return a.runPackage(ctx, args[1:])
	case "repo":
		return a.runRepo(ctx, args[1:])
	case "dotfile":
		return a.runDotfile(ctx, args[1:])
	case "add":
		return errors.New("`konen add` é ambíguo; use `konen project add [DIR]` para projetos ou `konen dotfile add CAMINHO...` para arquivos de configuração")
	case "trust":
		return a.runTrust(ctx, args[1:])
	case "doctor":
		return a.runDoctor(ctx)
	case "dev":
		return a.runDev(ctx, args[1:])
	case "project":
		return a.runProject(ctx, args[1:])
	case "projects":
		if len(args) != 1 {
			return errors.New("projects não aceita argumentos")
		}
		return a.runProject(ctx, []string{"list"})
	case "completion":
		return a.runCompletion(args[1:])
	case "__dotfile_add":
		return a.runDotfile(ctx, []string{"add"})
	case "__tool_add":
		return a.runTool(ctx, []string{"add"})
	case "__package_add":
		return a.runPackage(ctx, []string{"add"})
	case "__repo_add":
		return a.runRepo(ctx, []string{"add"})
	case "__plan_select":
		return a.runApply(ctx, []string{"--dry-run", "--select"})
	case "__apply_select":
		return a.runApply(ctx, []string{"--select"})
	case "__exit":
		return nil
	case "__complete":
		return a.runInternalComplete(args[1:])
	case "version", "--version", "-v":
		fmt.Fprintln(a.options.Out, a.options.Version)
		return nil
	case "help", "--help", "-h":
		a.printHelp()
		return nil
	default:
		return a.runProjectShortcut(ctx, args)
	}
}

func (a *App) runInit(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	initializeGit := flags.Bool("git", false, "inicializa um repositório Git")
	remote := flags.String("from", "", "clona um estado Git; use github:OWNER/REPO para login assistido")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("init aceita no máximo um caminho")
	}

	defaultPath := filepath.Join(a.options.HomeDir, ".local", "share", "konen", "state")
	answer := ui.InitAnswer{Path: defaultPath, Remote: *remote, InitializeGit: *initializeGit}
	if flags.NArg() == 1 {
		answer.Path = flags.Arg(0)
	} else if *remote == "" {
		if !a.options.Interactive {
			return errors.New("informe um caminho ou use --from em modo não interativo")
		}
		var err error
		answer, err = a.options.Prompter.Init(defaultPath)
		if err != nil {
			return err
		}
	}

	resolved, err := state.ResolvePath(answer.Path, a.options.HomeDir)
	if err != nil {
		return err
	}
	createdLocalConfig := false
	gitEnabled := false
	if answer.Remote != "" {
		err = a.cloneRemoteState(ctx, answer.Remote, resolved)
	} else {
		_, statErr := os.Stat(filepath.Join(resolved, "mise.toml"))
		createdLocalConfig = errors.Is(statErr, fs.ErrNotExist)
		err = a.state.PrepareLocal(ctx, resolved, answer.InitializeGit)
		if err == nil {
			gitEnabled = answer.InitializeGit
			if !gitEnabled {
				_, gitErr := os.Stat(filepath.Join(resolved, ".git"))
				gitEnabled = gitErr == nil
			}
		}
	}
	if err != nil {
		return err
	}
	if err := config.Save(a.options.ConfigPath, config.Config{StateDir: resolved}); err != nil {
		return err
	}

	misePath, miseErr := a.findCommand("mise")
	trusted := false
	if answer.Remote == "" && createdLocalConfig && miseErr == nil {
		miseConfig := filepath.Join(resolved, "mise.toml")
		if err := a.options.Runner.Run(ctx, resolved, misePath, "trust", miseConfig); err != nil {
			return fmt.Errorf("estado criado, mas não foi possível confiar no mise.toml: %w", err)
		}
		if _, err := a.stateTrust().Trust(resolved); err != nil {
			return fmt.Errorf("estado criado, mas não foi possível registrar a aprovação local: %w", err)
		}
		trusted = true
	}

	fmt.Fprintf(a.options.Out, "Konen configurado. Estado: %s\n", resolved)
	if answer.Remote == "" && gitEnabled {
		fmt.Fprintln(a.options.Out, "Git ativo. O Konen não cria commits; revise e versione as mudanças quando estiver pronto.")
	}
	switch {
	case trusted:
		fmt.Fprintln(a.options.Out, "Próximo passo: execute `konen apply`.")
	case miseErr != nil:
		fmt.Fprintln(a.options.Out, "Próximo passo: instale o mise, revise o mise.toml e execute `konen trust`.")
	default:
		fmt.Fprintln(a.options.Out, "Revise o mise.toml e execute `konen trust` antes de aplicar o estado.")
	}
	return nil
}

func (a *App) runApply(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	yes := flags.Bool("yes", false, "não pede confirmação")
	dryRun := flags.Bool("dry-run", false, "mostra o plano sem alterar a máquina")
	selectParts := flags.Bool("select", false, "escolhe as etapas interativamente")
	var only commaListFlag
	flags.Var(&only, "only", "limita a etapas separadas por vírgula")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("apply não aceita argumentos posicionais")
	}
	if *selectParts && len(only) > 0 {
		return errors.New("use apenas uma forma de seleção: --select ou --only")
	}
	if *selectParts && !a.options.Interactive {
		return errors.New("--select precisa de um terminal interativo; use --only ETAPAS em automações")
	}
	if *selectParts {
		selected, err := a.chooseApplyParts()
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			fmt.Fprintln(a.options.Out, "Nenhuma etapa selecionada; nada será alterado.")
			return nil
		}
		only = selected
	}
	if err := validateApplyParts(only); err != nil {
		return err
	}

	miseArgs := []string{"bootstrap"}
	if len(only) > 0 {
		miseArgs = append(miseArgs, "--only", strings.Join(only, ","))
		fmt.Fprintf(a.options.Out, "Etapas selecionadas: %s\n", strings.Join(applyPartLabels(only), ", "))
	}
	if *yes {
		miseArgs = append(miseArgs, "--yes")
	}
	if *dryRun {
		miseArgs = append(miseArgs, "--dry-run")
	}
	return a.runMise(ctx, miseArgs)
}

func (a *App) runDotfile(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add")
	}
	if args[0] != "add" {
		return fmt.Errorf("ação de dotfile desconhecida: %s", args[0])
	}
	return a.runDotfileAdd(ctx, args[1:])
}

func (a *App) runDotfileAdd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("dotfile add", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	mode := flags.String("mode", "", "modo do dotfile: symlink, copy ou template")
	if err := flags.Parse(args); err != nil {
		return err
	}
	targets := flags.Args()
	if len(targets) == 0 {
		if !a.options.Interactive {
			return errors.New("informe ao menos um arquivo de configuração")
		}
		target, err := a.options.Prompter.AddTarget()
		if err != nil {
			return err
		}
		if strings.TrimSpace(target) == "" {
			return errors.New("o caminho não pode ser vazio")
		}
		targets = []string{target}
	}
	for index, target := range targets {
		resolved, err := a.resolveDotfileTarget(target)
		if err != nil {
			return err
		}
		if err := a.validateDotfileTarget(resolved); err != nil {
			return err
		}
		targets[index] = resolved
	}

	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	miseArgs := []string{
		"bootstrap", "dotfiles", "add",
		"--path", filepath.Join(stateDir, "mise.toml"),
	}
	if *mode != "" {
		miseArgs = append(miseArgs, "--mode", *mode)
	}
	miseArgs = append(miseArgs, targets...)
	if err := a.runMise(ctx, miseArgs); err != nil {
		return err
	}
	// This guided mutation changes mise.toml by exactly the targets the user
	// supplied, so keep the local approval in sync with the resulting config.
	if _, err := a.stateTrust().Trust(stateDir); err != nil {
		return fmt.Errorf("dotfile adicionado, mas a aprovação local não pôde ser atualizada: %w", err)
	}
	return nil
}

func (a *App) validateDotfileTarget(target string) error {
	relative, err := filepath.Rel(filepath.Clean(a.options.HomeDir), filepath.Clean(target))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil
	}
	portable := filepath.ToSlash(relative)
	switch portable {
	case ".gitconfig":
		return errors.New("capturar ~/.gitconfig inteiro pode versionar credential helpers e caminhos desta máquina; mantenha preferências portáveis em ~/.config/git/config")
	case ".git-credentials", ".config/gh", ".config/gh/hosts.yml", ".netrc", ".ssh":
		return fmt.Errorf("%s contém credenciais e não pode ser capturado pelo Konen", target)
	default:
		if strings.HasPrefix(portable, ".ssh/id_") && !strings.HasSuffix(portable, ".pub") {
			return fmt.Errorf("%s parece ser uma chave SSH privada e não pode ser capturado pelo Konen", target)
		}
		return nil
	}
}

func (a *App) resolveDotfileTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("o caminho do dotfile não pode ser vazio")
	}
	if target == "~" || strings.HasPrefix(target, "~/") {
		return state.ResolvePath(target, a.options.HomeDir)
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target), nil
	}
	return filepath.Abs(filepath.Join(a.options.WorkDir, target))
}

func (a *App) runMise(ctx context.Context, args []string) error {
	stateDir, misePath, err := a.loadTrustedMise(ctx)
	if err != nil {
		return err
	}
	miseArgs := append([]string{"-C", stateDir}, args...)
	if err := a.options.Runner.RunEnv(ctx, stateDir, miseStateEnvironment(stateDir), misePath, miseArgs...); err != nil {
		return fmt.Errorf("mise: %w", err)
	}
	return nil
}

// miseStateEnvironment makes the selected Konen state the complete machine
// configuration for managed operations. Otherwise mise also merges a
// previously linked global config and files found in ancestor directories.
func miseStateEnvironment(stateDir string) []string {
	return []string{
		"MISE_GLOBAL_CONFIG_FILE=" + filepath.Join(stateDir, "mise.toml"),
		"MISE_GLOBAL_CONFIG_ROOT=" + stateDir,
		"MISE_CEILING_PATHS=" + stateDir,
		"MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml",
	}
}

func (a *App) loadTrustedMise(_ context.Context) (string, string, error) {
	stateDir, err := a.loadState()
	if err != nil {
		return "", "", err
	}
	trusted, err := a.stateTrust().IsTrusted(stateDir)
	if err != nil {
		return "", "", fmt.Errorf("não foi possível validar a aprovação local do estado: %w", err)
	}
	if !trusted {
		return "", "", fmt.Errorf("o mise.toml, um instalador ou um comando pessoal mudou; revise o estado e execute `konen trust`")
	}
	misePath, err := a.findCommand("mise")
	if err != nil {
		return "", "", errors.New("mise não está instalado; consulte https://mise.jdx.dev/installing-mise.html")
	}
	return stateDir, misePath, nil
}

func (a *App) runTrust(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("trust não aceita argumentos")
	}
	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	misePath, err := a.findCommand("mise")
	if err != nil {
		return errors.New("mise não está instalado; consulte https://mise.jdx.dev/installing-mise.html")
	}
	if _, _, _, err := state.ExecutionDigest(stateDir); err != nil {
		return fmt.Errorf("a superfície executável do estado não pode ser aprovada: %w", err)
	}
	miseConfig := filepath.Join(stateDir, "mise.toml")
	if err := a.options.Runner.Run(ctx, stateDir, misePath, "trust", miseConfig); err != nil {
		return fmt.Errorf("mise trust: %w", err)
	}
	files, err := a.stateTrust().Trust(stateDir)
	if err != nil {
		return fmt.Errorf("não foi possível registrar a aprovação local: %w", err)
	}
	fmt.Fprintf(a.options.Out, "Estado confiado: %s\n", miseConfig)
	fmt.Fprintf(a.options.Out, "Superfície executável aprovada: %d arquivo(s).\n", len(files))
	for _, file := range files {
		fmt.Fprintf(a.options.Out, "  %s\n", file)
	}
	return nil
}

func (a *App) runDoctor(ctx context.Context) error {
	healthy := true
	fmt.Fprintf(a.options.Out, "Sistema: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	stateDir, err := a.loadState()
	if err != nil {
		fmt.Fprintf(a.options.Out, "✗ configuração: %v\n", err)
		healthy = false
	} else {
		fmt.Fprintf(a.options.Out, "✓ configuração: %s\n", a.options.ConfigPath)
		fmt.Fprintf(a.options.Out, "✓ estado: %s\n", stateDir)
		trusted, trustErr := a.stateTrust().IsTrusted(stateDir)
		switch {
		case trustErr != nil:
			fmt.Fprintf(a.options.Out, "✗ confiança: %v\n", trustErr)
			healthy = false
		case trusted:
			fmt.Fprintln(a.options.Out, "✓ confiança: superfície executável aprovada")
		default:
			fmt.Fprintln(a.options.Out, "· confiança: revisão necessária; execute `konen trust`")
		}
	}

	if path, err := a.findCommand("mise"); err != nil {
		fmt.Fprintln(a.options.Out, "✗ mise: não encontrado")
		healthy = false
	} else {
		output, outputErr := a.options.Runner.Output(ctx, "", path, "--version")
		version, versionErr := extractVersion(output)
		switch {
		case outputErr != nil:
			fmt.Fprintf(a.options.Out, "✗ mise: não foi possível consultar a versão (%s)\n", path)
			healthy = false
		case versionErr != nil:
			fmt.Fprintf(a.options.Out, "✗ mise: versão não reconhecida (%s)\n", strings.TrimSpace(output))
			healthy = false
		case !versionAtLeast(version, minimumMiseVersion):
			fmt.Fprintf(a.options.Out, "✗ mise: %s; necessário >= %s\n", version, minimumMiseVersion)
			healthy = false
		default:
			fmt.Fprintf(a.options.Out, "✓ mise: %s (%s)\n", version, path)
		}
	}
	if path, err := a.options.Runner.LookPath("git"); err != nil {
		fmt.Fprintln(a.options.Out, "· git: não encontrado (opcional para estado local)")
	} else {
		fmt.Fprintf(a.options.Out, "✓ git: %s\n", path)
	}

	if !healthy {
		return errors.New("o diagnóstico encontrou problemas")
	}
	return nil
}

func (a *App) findCommand(name string) (string, error) {
	if a.options.BinDir != "" {
		candidate := filepath.Join(a.options.BinDir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return a.options.Runner.LookPath(name)
}

func extractVersion(output string) (string, error) {
	for _, field := range strings.Fields(output) {
		candidate := strings.TrimPrefix(field, "v")
		parts := strings.Split(candidate, ".")
		if len(parts) != 3 {
			continue
		}
		valid := true
		for _, part := range parts {
			if _, err := strconv.Atoi(part); err != nil {
				valid = false
				break
			}
		}
		if valid {
			return candidate, nil
		}
	}
	return "", errors.New("versão não encontrada")
}

func versionAtLeast(got, minimum string) bool {
	gotParts := strings.Split(got, ".")
	minimumParts := strings.Split(minimum, ".")
	if len(gotParts) != 3 || len(minimumParts) != 3 {
		return false
	}
	for index := range gotParts {
		gotNumber, gotErr := strconv.Atoi(gotParts[index])
		minimumNumber, minimumErr := strconv.Atoi(minimumParts[index])
		if gotErr != nil || minimumErr != nil {
			return false
		}
		if gotNumber != minimumNumber {
			return gotNumber > minimumNumber
		}
	}
	return true
}

func (a *App) loadState() (string, error) {
	cfg, err := config.Load(a.options.ConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", errors.New("Konen ainda não foi configurado; execute `konen init`")
	}
	if err != nil {
		return "", err
	}
	if err := state.Valid(cfg.StateDir); err != nil {
		return "", err
	}
	return cfg.StateDir, nil
}

func (a *App) stateTrust() state.TrustStore {
	return state.TrustStore{Path: filepath.Join(filepath.Dir(a.options.ConfigPath), "state-trust.toml")}
}

func (a *App) printHelp() {
	fmt.Fprintln(a.options.Out, "Konen — do zero à sua máquina")
	a.printCommandGroup("Início rápido", [][2]string{
		{"konen", "abre o menu interativo"},
		{"konen NOME", "abre um projeto cadastrado"},
		{"konen help", "mostra esta ajuda"},
	})
	a.printCommandGroup("Máquina", [][2]string{
		{"konen init [--git] [DIR]", "configura ou cria o estado"},
		{"konen init --from ORIGEM [DIR]", "clona um estado; GitHub privado tem login assistido"},
		{"konen status", "mostra tudo que o estado declara"},
		{"konen plan [--select]", "mostra o plano completo ou escolhe etapas"},
		{"konen plan --only ETAPAS", "mostra somente etapas separadas por vírgula"},
		{"konen apply [--select]", "aplica o estado completo ou escolhe etapas"},
		{"konen apply --only ETAPAS", "aplica somente etapas separadas por vírgula"},
		{"konen trust", "confia no mise.toml após revisão"},
		{"konen doctor", "diagnostica a instalação"},
	})
	a.printCommandGroup("Estado", [][2]string{
		{"konen tool add [NOME] [VERSÃO]", "adiciona uma ferramenta pelo assistente"},
		{"konen tool add --dry-run NOME [VERSÃO]", "mostra a alteração sem gravar"},
		{"konen package add [--manager M] PACOTE [VERSÃO]", "adiciona um pacote do sistema"},
		{"konen package add --dry-run [--manager M] PACOTE [VERSÃO]", "mostra a alteração sem gravar"},
		{"konen repo add DESTINO URL [REF]", "adiciona um repositório Git"},
		{"konen repo add --dry-run DESTINO URL [REF]", "mostra a alteração sem gravar"},
		{"konen dotfile add CAMINHO...", "adiciona dotfiles ao estado"},
		{"konen dotfile add --mode MODO CAMINHO...", "usa symlink, copy ou template"},
		{"konen diff", "mostra diferenças dos dotfiles"},
	})
	a.printCommandGroup("Projetos", [][2]string{
		{"konen projects", "lista os projetos cadastrados"},
		{"konen project add [DIR]", "cadastra um projeto e suas abas"},
		{"konen project edit NOME", "edita um projeto pelo assistente"},
		{"konen project show NOME", "mostra o manifesto de um projeto"},
		{"konen project trust NOME", "aprova os comandos após revisão"},
		{"konen dev [NOME] [--dry-run]", "abre ou inspeciona a sessão do projeto"},
	})
	a.printCommandGroup("Shell", [][2]string{
		{"konen completion zsh|bash|fish", "gera o autocomplete"},
		{"konen version", "mostra a versão instalada"},
	})
}

func (a *App) printCommandGroup(title string, commands [][2]string) {
	fmt.Fprintf(a.options.Out, "\n%s:\n", title)
	width := 0
	for _, command := range commands {
		if len(command[0]) > width {
			width = len(command[0])
		}
	}
	for _, command := range commands {
		fmt.Fprintf(a.options.Out, "  %s\n", ui.CommandLabel(command[0], command[1], width))
	}
}
