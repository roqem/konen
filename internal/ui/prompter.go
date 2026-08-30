package ui

import (
	"io"

	"charm.land/huh/v2"
)

type InitAnswer struct {
	Path          string
	Remote        string
	InitializeGit bool
}

type Prompter interface {
	Menu(configured bool) (string, error)
	Init(defaultPath string) (InitAnswer, error)
	AddTarget() (string, error)
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
		huh.NewOption("Ver o estado", "status"),
		huh.NewOption("Adicionar um arquivo", "add"),
		huh.NewOption("Diagnosticar problemas", "doctor"),
	}
	if !configured {
		options = []huh.Option[string]{
			huh.NewOption("Configurar o Zeroot", "init"),
			huh.NewOption("Diagnosticar problemas", "doctor"),
		}
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Zeroot — do zero à sua máquina").
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
