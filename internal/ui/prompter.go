package ui

import (
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

type Prompter interface {
	Menu(configured bool) (string, error)
	Init(defaultPath string) (InitAnswer, error)
	AddTarget() (string, error)
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

func (p HuhPrompter) Menu(configured bool) (string, error) {
	var action string
	options := []huh.Option[string]{
		huh.NewOption("Preparar esta máquina", "apply"),
		huh.NewOption("Abrir um projeto", "dev"),
		huh.NewOption("Ver o estado", "status"),
		huh.NewOption("Adicionar um arquivo", "add"),
		huh.NewOption("Confiar no estado após revisar", "trust"),
		huh.NewOption("Diagnosticar problemas", "doctor"),
	}
	if !configured {
		options = []huh.Option[string]{
			huh.NewOption("Configurar o Konen", "init"),
			huh.NewOption("Diagnosticar problemas", "doctor"),
		}
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Konen — do zero à sua máquina").
			Options(options...).
			Value(&action),
	)).WithInput(p.in).WithOutput(p.out)
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
	)).WithInput(p.in).WithOutput(p.out)
	if err := first.Run(); err != nil {
		return InitAnswer{}, err
	}

	fields := []huh.Field{
		huh.NewInput().Title("Caminho do estado").Value(&answer.Path),
	}
	if source == "remote" {
		fields = append([]huh.Field{
			huh.NewInput().Title("URL do repositório Git").Value(&answer.Remote),
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

	form := huh.NewForm(huh.NewGroup(fields...)).WithInput(p.in).WithOutput(p.out)
	return answer, form.Run()
}

func (p HuhPrompter) AddTarget() (string, error) {
	var target string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Arquivo ou diretório para adicionar").Value(&target),
	)).WithInput(p.in).WithOutput(p.out)
	return target, form.Run()
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
	)).WithInput(p.in).WithOutput(p.out)
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
		)).WithInput(p.in).WithOutput(p.out)
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
		)).WithInput(p.in).WithOutput(p.out)
		if err := form.Run(); err != nil {
			return ProjectAnswer{}, err
		}
	}
	for addAnother {
		tab := ProjectTabAnswer{}
		if len(tabs) == 0 {
			tab = ProjectTabAnswer{Title: "Editor", Command: "nvim ."}
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
		)).WithInput(p.in).WithOutput(p.out)
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
	)).WithInput(p.in).WithOutput(p.out)
	return selected, form.Run()
}
