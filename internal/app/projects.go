package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/ui"
)

func (a *App) runProject(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add, edit, list, show, trust ou run")
	}
	if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
		return a.printProjectActionHelp(args[0])
	}
	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	store := project.Store{StateDir: stateDir, HomeDir: a.options.HomeDir}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("project list não aceita argumentos")
		}
		projects, err := store.List()
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Fprintln(a.options.Out, "Nenhum projeto cadastrado.")
			return nil
		}
		trust := a.projectTrust()
		rows := make([][]string, 0, len(projects))
		for _, item := range projects {
			trusted, err := trust.IsTrusted(item.Path)
			if err != nil {
				return err
			}
			approval := "revisão necessária"
			if trusted {
				approval = "aprovado"
			}
			rows = append(rows, []string{item.Name, approval, item.Manifest.Path})
		}
		fmt.Fprint(a.options.Out, ui.RenderTable([]string{"Projeto", "Aprovação", "Pasta"}, rows))
		return nil
	case "show":
		if len(args) != 2 {
			return errors.New("uso: konen project show NOME")
		}
		_, path, err := store.Load(args[1])
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = a.options.Out.Write(data)
		return err
	case "trust":
		if len(args) != 2 {
			return errors.New("uso: konen project trust NOME")
		}
		manifest, path, err := store.Load(args[1])
		if err != nil {
			return err
		}
		projectDir, err := store.ResolveProjectPath(manifest)
		if err != nil {
			return err
		}
		a.printProjectPlan(args[1], projectDir, manifest)
		if err := a.projectTrust().Trust(path); err != nil {
			return err
		}
		fmt.Fprintf(a.options.Out, "Projeto aprovado: %s\n", args[1])
		return nil
	case "add":
		return a.runProjectAdd(ctx, store, args[1:])
	case "edit":
		return a.runProjectEdit(ctx, store, args[1:])
	case "run":
		return a.runNamedProjectAction(ctx, args[1:], true)
	default:
		return fmt.Errorf("ação de projeto desconhecida: %s", args[0])
	}
}

func (a *App) printProjectActionHelp(action string) error {
	var usage string
	switch action {
	case "add":
		usage = "konen project add [DIR]"
	case "edit":
		usage = "konen project edit NOME"
	case "list":
		usage = "konen project list"
	case "show":
		usage = "konen project show NOME"
	case "trust":
		usage = "konen project trust NOME"
	case "run":
		usage = "konen project run NOME AÇÃO [--dry-run]"
	default:
		return fmt.Errorf("ação de projeto desconhecida: %s", action)
	}
	a.printCommandGroup("Uso", [][2]string{{usage, "mostra ou executa esta operação de projeto"}})
	return nil
}

func (a *App) runProjectShortcut(ctx context.Context, args []string) error {
	name := args[0]
	if err := project.ValidateName(name); err != nil {
		return fmt.Errorf("comando desconhecido: %s; execute `konen help`", name)
	}
	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	store := project.Store{StateDir: stateDir, HomeDir: a.options.HomeDir}
	if _, _, err := store.Load(name); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%q não é um comando nem um projeto cadastrado; consulte `konen projects` ou cadastre-o com `konen project add [DIR]`", name)
		}
		return err
	}
	return a.runDev(ctx, args)
}

func (a *App) runProjectAdd(_ context.Context, store project.Store, args []string) error {
	if len(args) > 1 {
		return errors.New("project add aceita no máximo um caminho")
	}
	if !a.options.Interactive {
		return errors.New("project add requer uma sessão interativa")
	}
	projectDir := a.options.WorkDir
	if len(args) == 1 {
		projectDir = args[0]
	}
	resolved, err := project.ResolvePath(projectDir, a.options.HomeDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("o projeto não é um diretório: %s", resolved)
	}

	answer, err := a.options.Prompter.Project(ui.ProjectAnswer{
		Name:            strings.ToLower(filepath.Base(resolved)),
		Path:            resolved,
		KeepInvokingTab: true,
	})
	if err != nil {
		return err
	}
	if err := project.ValidateName(answer.Name); err != nil {
		return err
	}
	manifestPath := store.ManifestPath(answer.Name)
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("o projeto %q já existe; use `konen project edit %s`", answer.Name, answer.Name)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	manifest, err := a.manifestFromAnswer(answer)
	if err != nil {
		return err
	}
	path, err := store.Save(answer.Name, manifest)
	if err != nil {
		return err
	}
	if err := a.projectTrust().Trust(path); err != nil {
		return err
	}
	fmt.Fprintf(a.options.Out, "Projeto cadastrado e aprovado: %s (%s)\n", answer.Name, path)
	return nil
}

func (a *App) runProjectEdit(_ context.Context, store project.Store, args []string) error {
	if len(args) != 1 {
		return errors.New("uso: konen project edit NOME")
	}
	if !a.options.Interactive {
		return errors.New("project edit requer uma sessão interativa")
	}
	name := args[0]
	manifest, _, err := store.Load(name)
	if err != nil {
		return err
	}
	resolved, err := store.ResolveProjectPath(manifest)
	if err != nil {
		return err
	}
	answer := ui.ProjectAnswer{
		Name: name, Path: resolved, Shell: manifest.Shell,
		KeepInvokingTab: manifest.KeepsInvokingTab(),
	}
	actionNames := make([]string, 0, len(manifest.Actions))
	for actionName := range manifest.Actions {
		actionNames = append(actionNames, actionName)
	}
	sort.Strings(actionNames)
	for _, actionName := range actionNames {
		answer.Actions = append(answer.Actions, ui.ProjectActionAnswer{
			Name: actionName, Task: manifest.Actions[actionName].Task,
		})
	}
	for _, tab := range manifest.Tabs {
		answer.Tabs = append(answer.Tabs, ui.ProjectTabAnswer{
			Title: tab.Title, Command: tab.Command, Action: tab.Action, Hold: tab.Hold,
		})
	}
	answer, err = a.options.Prompter.Project(answer)
	if err != nil {
		return err
	}
	if answer.Name != name {
		return errors.New("renomear projetos ainda não é suportado; mantenha o nome atual")
	}
	updated, err := a.manifestFromAnswer(answer)
	if err != nil {
		return err
	}
	path, err := store.Save(name, updated)
	if err != nil {
		return err
	}
	if err := a.projectTrust().Trust(path); err != nil {
		return err
	}
	fmt.Fprintf(a.options.Out, "Projeto atualizado e aprovado: %s\n", name)
	return nil
}

func (a *App) manifestFromAnswer(answer ui.ProjectAnswer) (project.Manifest, error) {
	resolved, err := project.ResolvePath(answer.Path, a.options.HomeDir)
	if err != nil {
		return project.Manifest{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return project.Manifest{}, fmt.Errorf("pasta do projeto indisponível: %w", err)
	}
	if !info.IsDir() {
		return project.Manifest{}, fmt.Errorf("o projeto não é um diretório: %s", resolved)
	}
	portable, err := project.PortablePath(resolved, a.options.HomeDir)
	if err != nil {
		return project.Manifest{}, err
	}
	keepInvokingTab := answer.KeepInvokingTab
	manifest := project.Manifest{
		Version: 2, Path: portable, Shell: answer.Shell,
		KeepInvokingTab: &keepInvokingTab,
		Actions:         make(map[string]project.Action, len(answer.Actions)),
	}
	for _, action := range answer.Actions {
		if _, exists := manifest.Actions[action.Name]; exists {
			return project.Manifest{}, fmt.Errorf("ação repetida: %s", action.Name)
		}
		manifest.Actions[action.Name] = project.Action{Task: action.Task}
	}
	for _, tab := range answer.Tabs {
		manifest.Tabs = append(manifest.Tabs, project.Tab{
			Title: tab.Title, Command: tab.Command, Action: tab.Action, Hold: tab.Hold,
		})
	}
	return manifest, project.Validate(manifest)
}

func (a *App) runNamedProjectAction(ctx context.Context, args []string, projectRequired bool) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			usage := "konen run [PROJETO] AÇÃO [--dry-run]"
			if projectRequired {
				usage = "konen project run NOME AÇÃO [--dry-run]"
			}
			a.printCommandGroup("Uso", [][2]string{{usage, "executa ou inspeciona uma ação"}})
			return nil
		}
	}
	positionals, dryRun, err := parseProjectActionArgs(args)
	if err != nil {
		return err
	}
	if projectRequired && len(positionals) != 2 {
		return errors.New("uso: konen project run NOME AÇÃO [--dry-run]")
	}
	if !projectRequired && len(positionals) > 2 {
		return errors.New("uso: konen run [PROJETO] AÇÃO [--dry-run]")
	}
	if !projectRequired && len(positionals) == 0 && !a.options.Interactive {
		return errors.New("uso: konen run [PROJETO] AÇÃO [--dry-run]")
	}

	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	store := project.Store{StateDir: stateDir, HomeDir: a.options.HomeDir}
	var projectName, actionName string
	if len(positionals) == 2 {
		projectName, actionName = positionals[0], positionals[1]
	} else {
		if len(positionals) == 1 {
			actionName = positionals[0]
		}
		projectName, err = a.selectProject(store)
		if err != nil {
			return err
		}
	}
	manifest, manifestPath, err := store.Load(projectName)
	if err != nil {
		return err
	}
	if actionName == "" {
		actionName, err = a.options.Prompter.ChooseProjectAction(projectName, sortedActionNames(manifest))
		if err != nil {
			return err
		}
	}
	action, ok := manifest.Actions[actionName]
	if !ok {
		return fmt.Errorf("o projeto %q não possui a ação %q%s", projectName, actionName, availableActions(manifest))
	}
	projectDir, err := store.ResolveProjectPath(manifest)
	if err != nil {
		return err
	}
	info, err := os.Stat(projectDir)
	if err != nil {
		return fmt.Errorf("pasta do projeto indisponível: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("a pasta do projeto não é um diretório: %s", projectDir)
	}
	trusted, err := a.projectTrust().IsTrusted(manifestPath)
	if err != nil {
		return err
	}
	printProjectActionPlan(a.options.Out, projectName, projectDir, actionName, action.Task)
	if dryRun {
		if trusted {
			fmt.Fprintln(a.options.Out, "Aprovação local: válida")
		} else {
			fmt.Fprintf(a.options.Out, "Aprovação local: pendente — revise e execute `konen project trust %s`\n", projectName)
		}
		fmt.Fprintln(a.options.Out, "Nenhuma tarefa foi executada.")
		return nil
	}
	if !trusted {
		return fmt.Errorf("as ações de %q ainda não foram aprovadas ou mudaram; revise com `konen project show %s` e execute `konen project trust %s`", projectName, projectName, projectName)
	}
	misePath, err := a.findCommand("mise")
	if err != nil {
		return errors.New("mise não foi encontrado; execute `konen doctor`")
	}
	if err := a.options.Runner.Run(ctx, projectDir, misePath, "run", "--raw", action.Task); err != nil {
		return fmt.Errorf("a ação %q falhou: %w", actionName, err)
	}
	return nil
}

func parseProjectActionArgs(args []string) ([]string, bool, error) {
	positionals := make([]string, 0, 2)
	var dryRun bool
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, false, fmt.Errorf("opção desconhecida: %s", arg)
			}
			positionals = append(positionals, arg)
		}
	}
	return positionals, dryRun, nil
}

func printProjectActionPlan(out io.Writer, projectName, dir, actionName, task string) {
	fmt.Fprintf(out, "Projeto: %s\nPasta: %s\n", projectName, dir)
	fmt.Fprint(out, ui.RenderTable(
		[]string{"Ação", "Tarefa do mise", "Execução"},
		[][]string{{actionName, task, "mise run --raw " + task}},
	))
}

func availableActions(manifest project.Manifest) string {
	names := sortedActionNames(manifest)
	if len(names) == 0 {
		return "; nenhuma ação foi cadastrada"
	}
	return "; disponíveis: " + strings.Join(names, ", ")
}

func sortedActionNames(manifest project.Manifest) []string {
	names := make([]string, 0, len(manifest.Actions))
	for name := range manifest.Actions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (a *App) runDev(ctx context.Context, args []string) error {
	name, dryRun, err := parseDevArgs(args)
	if err != nil {
		return err
	}
	stateDir, err := a.loadState()
	if err != nil {
		return err
	}
	store := project.Store{StateDir: stateDir, HomeDir: a.options.HomeDir}
	if name == "" {
		name, err = a.selectProject(store)
		if err != nil {
			return err
		}
	}
	manifest, manifestPath, err := store.Load(name)
	if err != nil {
		return err
	}
	projectDir, err := store.ResolveProjectPath(manifest)
	if err != nil {
		return err
	}
	info, err := os.Stat(projectDir)
	if err != nil {
		return fmt.Errorf("pasta do projeto indisponível: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("a pasta do projeto não é um diretório: %s", projectDir)
	}
	trusted, err := a.projectTrust().IsTrusted(manifestPath)
	if err != nil {
		return err
	}
	if dryRun {
		a.printProjectPlan(name, projectDir, manifest)
		if trusted {
			fmt.Fprintln(a.options.Out, "Aprovação local: válida")
		} else {
			fmt.Fprintf(a.options.Out, "Aprovação local: pendente — revise e execute `konen project trust %s`\n", name)
		}
		return nil
	}
	if !trusted {
		return fmt.Errorf("os comandos de %q ainda não foram aprovados ou mudaram; revise com `konen project show %s` e execute `konen project trust %s`", name, name, name)
	}

	if a.options.Getenv("KITTY_WINDOW_ID") != "" {
		return a.launchInCurrentKitty(ctx, name, projectDir, manifest)
	}
	return a.launchKittySession(ctx, name, projectDir, manifest)
}

func parseDevArgs(args []string) (string, bool, error) {
	var name string
	var dryRun bool
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			return "", false, errors.New("uso: konen dev [NOME] [--dry-run]")
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, fmt.Errorf("opção desconhecida: %s", arg)
			}
			if name != "" {
				return "", false, errors.New("dev aceita no máximo um nome de projeto")
			}
			name = arg
		}
	}
	return name, dryRun, nil
}

func (a *App) selectProject(store project.Store) (string, error) {
	matches, err := store.MatchDirectory(a.options.WorkDir)
	if err != nil {
		return "", err
	}
	if len(matches) > 0 {
		return matches[0].Name, nil
	}
	if !a.options.Interactive {
		return "", errors.New("não foi possível inferir o projeto atual; informe o nome")
	}
	projects, err := store.List()
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(projects))
	for _, item := range projects {
		names = append(names, item.Name)
	}
	return a.options.Prompter.ChooseProject(names)
}

func (a *App) launchInCurrentKitty(ctx context.Context, name, dir string, manifest project.Manifest) error {
	kitten, err := a.findCommand("kitten")
	if err != nil {
		return errors.New("kitten não foi encontrado; instale o Kitty ou execute `konen dev --dry-run`")
	}
	if _, err := a.options.Runner.Output(ctx, dir, kitten, "@", "ls"); err != nil {
		return fmt.Errorf("controle remoto do Kitty indisponível; execute diretamente em uma aba do Kitty e confirme `allow_remote_control yes` no kitty.conf: %w", err)
	}
	misePath, err := a.miseForProjectTabs(manifest)
	if err != nil {
		return err
	}

	shell := a.projectShell(manifest)
	var firstWindowID string
	for _, tab := range manifest.Tabs {
		launchArgs := []string{
			"@", "launch", "--self", "--type=tab", "--keep-focus",
			"--tab-title", tab.Title, "--cwd", dir,
			"--add-to-session", "konen-" + name,
		}
		if tab.Hold {
			launchArgs = append(launchArgs, "--hold")
		}
		launchArgs = append(launchArgs, shell)
		command := projectTabCommand(manifest, tab, misePath)
		if command == "" {
			launchArgs = append(launchArgs, "-l")
		} else {
			launchArgs = append(launchArgs, "-lic", command)
		}
		output, err := a.options.Runner.Output(ctx, dir, kitten, launchArgs...)
		if err != nil {
			return fmt.Errorf("não foi possível abrir a aba %q: %w", tab.Title, err)
		}
		if firstWindowID == "" {
			fields := strings.Fields(output)
			if len(fields) == 0 {
				return fmt.Errorf("Kitty não informou o id da aba %q", tab.Title)
			}
			firstWindowID = fields[0]
		}
	}
	if err := a.options.Runner.Run(ctx, dir, kitten, "@", "focus-tab", "--match", "window_id:"+firstWindowID); err != nil {
		return fmt.Errorf("as abas foram abertas, mas não foi possível focar a primeira: %w", err)
	}
	fmt.Fprintf(a.options.Out, "Projeto aberto na janela atual do Kitty: %s\n", name)
	if !manifest.KeepsInvokingTab() {
		if err := a.options.Runner.Run(ctx, dir, kitten, "@", "close-window", "--self", "--no-response"); err != nil {
			return fmt.Errorf("o projeto foi aberto, mas não foi possível fechar o terminal invocador: %w", err)
		}
	}
	return nil
}

func (a *App) launchKittySession(ctx context.Context, name, dir string, manifest project.Manifest) error {
	kitty, err := a.findCommand("kitty")
	if err != nil {
		return errors.New("Kitty não foi encontrado; execute `konen dev --dry-run` para revisar a sessão")
	}
	misePath, err := a.miseForProjectTabs(manifest)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp("", "konen-*.kitty-session")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteString(renderKittySession(dir, a.projectShell(manifest), misePath, manifest)); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := a.options.Runner.Run(ctx, dir, kitty, "--detach", "--session", path); err != nil {
		return fmt.Errorf("não foi possível abrir o projeto no Kitty: %w", err)
	}
	fmt.Fprintf(a.options.Out, "Projeto aberto em uma nova janela do Kitty: %s\n", name)
	return nil
}

func (a *App) projectShell(manifest project.Manifest) string {
	if manifest.Shell != "" {
		return manifest.Shell
	}
	if shell := a.options.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func (a *App) miseForProjectTabs(manifest project.Manifest) (string, error) {
	for _, tab := range manifest.Tabs {
		if tab.Action == "" {
			continue
		}
		misePath, err := a.findCommand("mise")
		if err != nil {
			return "", errors.New("uma aba usa uma ação, mas mise não foi encontrado; execute `konen doctor`")
		}
		return misePath, nil
	}
	return "", nil
}

func (a *App) projectTrust() project.TrustStore {
	return project.TrustStore{Path: filepath.Join(filepath.Dir(a.options.ConfigPath), "projects-trust.toml")}
}

func (a *App) printProjectPlan(name, dir string, manifest project.Manifest) {
	destination := "nova janela do Kitty"
	insideKitty := a.options.Getenv("KITTY_WINDOW_ID") != ""
	if insideKitty {
		destination = "abas na janela atual do Kitty"
	}
	fmt.Fprintf(a.options.Out, "Projeto: %s\nPasta: %s\nDestino: %s\n", name, dir, destination)
	if insideKitty {
		invokingTab := "manter"
		if !manifest.KeepsInvokingTab() {
			invokingTab = "fechar"
		}
		fmt.Fprintf(a.options.Out, "Aba invocadora: %s\n", invokingTab)
	}
	if len(manifest.Actions) > 0 {
		names := make([]string, 0, len(manifest.Actions))
		for actionName := range manifest.Actions {
			names = append(names, actionName)
		}
		sort.Strings(names)
		rows := make([][]string, 0, len(names))
		for _, actionName := range names {
			rows = append(rows, []string{actionName, manifest.Actions[actionName].Task})
		}
		fmt.Fprintln(a.options.Out, "Ações nomeadas:")
		fmt.Fprint(a.options.Out, ui.RenderTable([]string{"Ação", "Tarefa do mise"}, rows))
	}
	rows := make([][]string, 0, len(manifest.Tabs))
	for _, tab := range manifest.Tabs {
		command := projectTabDescription(manifest, tab)
		if command == "" {
			command = "<shell>"
		}
		afterExit := "fechar"
		if tab.Hold {
			afterExit = "manter"
		}
		rows = append(rows, []string{tab.Title, command, afterExit})
	}
	fmt.Fprint(a.options.Out, ui.RenderTable([]string{"Aba", "Comando", "Após sair"}, rows))
}

func renderKittySession(dir, shell, misePath string, manifest project.Manifest) string {
	var output strings.Builder
	for _, tab := range manifest.Tabs {
		fmt.Fprintf(&output, "new_tab %s\n", shellQuote(tab.Title))
		fmt.Fprintf(&output, "cd %s\n", shellQuote(dir))
		output.WriteString("launch")
		if tab.Hold {
			output.WriteString(" --hold")
		}
		fmt.Fprintf(&output, " %s", shellQuote(shell))
		command := projectTabCommand(manifest, tab, misePath)
		if command == "" {
			output.WriteString(" -l\n\n")
		} else {
			fmt.Fprintf(&output, " -lic %s\n\n", shellQuote(command))
		}
	}
	output.WriteString("focus_tab 0\n")
	return output.String()
}

func projectTabDescription(manifest project.Manifest, tab project.Tab) string {
	if tab.Action == "" {
		return tab.Command
	}
	return fmt.Sprintf("ação %s → mise run --raw %s", tab.Action, manifest.Actions[tab.Action].Task)
}

func projectTabCommand(manifest project.Manifest, tab project.Tab, misePath string) string {
	if tab.Action == "" {
		return tab.Command
	}
	return fmt.Sprintf("%s run --raw %s", shellQuote(misePath), shellQuote(manifest.Actions[tab.Action].Task))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
