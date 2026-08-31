package ui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/huh/v2"
)

type InitAnswer struct {
	Path          string
	Remote        string
	InitializeGit bool
}

type ProjectTabAnswer struct {
	Title   string
	Command string
	Hold    bool
}

type ProjectAnswer struct {
	Name            string
	Path            string
	Shell           string
	KeepInvokingTab bool
	Tabs            []ProjectTabAnswer
}

type ToolAnswer struct {
	Name    string
	Version string
}

type Prompter interface {
	Menu(configured bool) (string, error)
	Init(defaultPath string) (InitAnswer, error)
	AddTarget() (string, error)
	Tool(ToolAnswer) (ToolAnswer, error)
	Confirm(string) (bool, error)
	Project(ProjectAnswer) (ProjectAnswer, error)
	ChooseProject([]string) (string, error)
}

type HuhPrompter struct {
	in  io.Reader
	out io.Writer
}

func NewHuhPrompter(in io.Reader, out io.Writer) HuhPrompter {
	return HuhPrompter{in: in, out: out}
}

func IsUserAborted(err error) bool {
	return errors.Is(err, huh.ErrUserAborted)
}

func cancelKeyMap() *huh.KeyMap {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Quit.SetKeys("ctrl+c", "esc")
	keyMap.Quit.SetHelp("esc", "sair")
	return keyMap
}

func menuKeyMap() *huh.KeyMap {
	keyMap := cancelKeyMap()
	keyMap.Quit.SetKeys("ctrl+c", "esc", "q", "Q")
	keyMap.Quit.SetHelp("q/esc", "sair")
	return keyMap
}

func (p HuhPrompter) Menu(configured bool) (string, error) {
	var action string
	options := []huh.Option[string]{
		huh.NewOption(CommandLabel("plan", "revisar o que mudaria", 13), "plan"),
		huh.NewOption(CommandLabel("apply", "preparar esta máquina", 13), "apply"),
		huh.NewOption(CommandLabel("dev", "abrir um projeto", 13), "dev"),
		huh.NewOption(CommandLabel("status", "ver tudo configurado", 13), "status"),
		huh.NewOption(CommandLabel("tool add", "adicionar uma ferramenta", 13), "__tool_add"),
		huh.NewOption(CommandLabel("dotfile add", "adicionar um arquivo de configuração", 13), "__dotfile_add"),
		huh.NewOption(CommandLabel("trust", "confiar no estado após revisar", 13), "trust"),
		huh.NewOption(CommandLabel("doctor", "diagnosticar problemas", 13), "doctor"),
		huh.NewOption(CommandLabel("q", "sair", 13), "__exit"),
	}
	if !configured {
		options = []huh.Option[string]{
			huh.NewOption(CommandLabel("init", "configurar o Konen", 8), "init"),
			huh.NewOption(CommandLabel("doctor", "diagnosticar problemas", 8), "doctor"),
			huh.NewOption(CommandLabel("q", "sair", 8), "__exit"),
		}
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Konen — do zero à sua máquina").
			Options(options...).
			Value(&action),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(menuKeyMap())
	return action, form.Run()
}

func (p HuhPrompter) Init(defaultPath string) (InitAnswer, error) {
	answer := InitAnswer{Path: defaultPath}
	var source string

	first := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Onde está seu estado?").
			Options(
				huh.NewOption("Criar ou usar uma pasta local", "local"),
				huh.NewOption("Clonar um repositório Git", "remote"),
			).
			Value(&source),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	if err := first.Run(); err != nil {
		return InitAnswer{}, err
	}

	fields := []huh.Field{
		huh.NewInput().Title("Caminho do estado").Value(&answer.Path),
	}
	if source == "remote" {
		fields = append([]huh.Field{
			huh.NewInput().
				Title("Origem do repositório Git").
				Description("Use github:OWNER/REPO para login HTTPS assistido.").
				Value(&answer.Remote),
		}, fields...)
	} else {
		fields = append(fields,
			huh.NewConfirm().
				Title("Inicializar Git se necessário?").
				Affirmative("Sim").
				Negative("Não").
				Value(&answer.InitializeGit),
		)
	}

	form := huh.NewForm(huh.NewGroup(fields...)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	return answer, form.Run()
}

func (p HuhPrompter) AddTarget() (string, error) {
	var target string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Dotfile para adicionar").Value(&target),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	return target, form.Run()
}

func (p HuhPrompter) Tool(answer ToolAnswer) (ToolAnswer, error) {
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Ferramenta").
			Description("Nome reconhecido pelo mise, como node, ruby ou neovim.").
			Value(&answer.Name),
		huh.NewInput().
			Title("Versão").
			Description("Use latest, lts ou uma versão como 3.4.").
			Value(&answer.Version),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	if err := form.Run(); err != nil {
		return ToolAnswer{}, err
	}
	answer.Name = strings.TrimSpace(answer.Name)
	answer.Version = strings.TrimSpace(answer.Version)
	return answer, nil
}

func (p HuhPrompter) Confirm(title string) (bool, error) {
	confirmed := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Affirmative("Gravar").
			Negative("Cancelar").
			Value(&confirmed),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	return confirmed, form.Run()
}

func (p HuhPrompter) Project(answer ProjectAnswer) (ProjectAnswer, error) {
	identity := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Nome curto do projeto").Value(&answer.Name),
		huh.NewInput().Title("Pasta do projeto").Value(&answer.Path),
		huh.NewInput().
			Title("Shell (opcional)").
			Description("Vazio usa $SHELL; os comandos carregam o ambiente interativo.").
			Value(&answer.Shell),
		huh.NewConfirm().
			Title("Manter a aba que executou `konen dev`?").
			Affirmative("Manter").Negative("Fechar").Value(&answer.KeepInvokingTab),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	if err := identity.Run(); err != nil {
		return ProjectAnswer{}, err
	}

	tabs := make([]ProjectTabAnswer, 0, len(answer.Tabs)+1)
	for index, existing := range answer.Tabs {
		keep := true
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title(fmt.Sprintf("Aba %d — título", index+1)).Value(&existing.Title),
			huh.NewInput().
				Title("Comando").
				Description("Vazio abre apenas o shell do projeto.").
				Value(&existing.Command),
			huh.NewConfirm().
				Title("Manter aberta quando o comando terminar?").
				Affirmative("Sim").Negative("Não").Value(&existing.Hold),
			huh.NewConfirm().
				Title("Manter esta aba?").
				Affirmative("Sim").Negative("Remover").Value(&keep),
		)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
		if err := form.Run(); err != nil {
			return ProjectAnswer{}, err
		}
		if keep {
			existing.Title = strings.TrimSpace(existing.Title)
			existing.Command = strings.TrimSpace(existing.Command)
			tabs = append(tabs, existing)
		}
	}

	addAnother := len(tabs) == 0
	if len(tabs) > 0 {
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Adicionar outra aba?").
				Affirmative("Sim").Negative("Não").Value(&addAnother),
		)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
		if err := form.Run(); err != nil {
			return ProjectAnswer{}, err
		}
	}
	for addAnother {
		tab := ProjectTabAnswer{}
		if len(tabs) == 0 {
			tab = defaultProjectTab()
		}
		addAnother = false
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Título da aba").Value(&tab.Title),
			huh.NewInput().
				Title("Comando").
				Description("Vazio abre apenas o shell do projeto.").
				Value(&tab.Command),
			huh.NewConfirm().
				Title("Manter aberta quando o comando terminar?").
				Affirmative("Sim").Negative("Não").Value(&tab.Hold),
			huh.NewConfirm().
				Title("Adicionar mais uma aba?").
				Affirmative("Sim").Negative("Não").Value(&addAnother),
		)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
		if err := form.Run(); err != nil {
			return ProjectAnswer{}, err
		}
		tab.Title = strings.TrimSpace(tab.Title)
		tab.Command = strings.TrimSpace(tab.Command)
		tabs = append(tabs, tab)
	}

	answer.Name = strings.TrimSpace(answer.Name)
	answer.Path = strings.TrimSpace(answer.Path)
	answer.Shell = strings.TrimSpace(answer.Shell)
	answer.Tabs = tabs
	return answer, nil
}

func defaultProjectTab() ProjectTabAnswer {
	return ProjectTabAnswer{Title: "Terminal"}
}

func (p HuhPrompter) ChooseProject(names []string) (string, error) {
	if len(names) == 0 {
		return "", fmt.Errorf("nenhum projeto cadastrado; execute `konen project add`")
	}
	options := make([]huh.Option[string], 0, len(names))
	for _, name := range names {
		options = append(options, huh.NewOption(name, name))
	}
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Qual projeto deseja abrir?").Options(options...).Value(&selected),
	)).WithInput(p.in).WithOutput(p.out).WithKeyMap(cancelKeyMap())
	return selected, form.Run()
}
