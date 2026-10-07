package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"github.com/roqem/konen/internal/project"
)

type Choice struct {
	Value string
	Label string
}

type ProjectServices struct {
	Tasks    func(string) ([]project.Task, error)
	EditTask func(string, project.Task) error
}

func (p HuhPrompter) Choose(title string, choices []Choice) (string, error) {
	return p.chooseValue(title, choices, "")
}

func (p HuhPrompter) chooseValue(title string, choices []Choice, selected string) (string, error) {
	options := make([]huh.Option[string], 0, len(choices))
	for _, choice := range choices {
		options = append(options, huh.NewOption(choice.Label, choice.Value))
	}
	err := p.projectForm(huh.NewSelect[string]().Title(title).
		Description("↑/↓ para escolher · / para filtrar · Enter para abrir · Esc para voltar").
		Options(options...).Height(min(len(options)+2, 18)).Value(&selected))
	return selected, err
}

func (p HuhPrompter) projectForm(fields ...huh.Field) error {
	return huh.NewForm(huh.NewGroup(fields...)).WithInput(p.in).
		WithOutput(p.out).WithKeyMap(cancelKeyMap()).Run()
}

func (p HuhPrompter) Project(answer ProjectAnswer, services ProjectServices) (ProjectAnswer, error) {
	answer.Tabs = append([]ProjectTabAnswer(nil), answer.Tabs...)
	answer.Actions = append([]ProjectActionAnswer(nil), answer.Actions...)
	creating := len(answer.Tabs) == 0
	if creating {
		if err := p.projectIdentity(&answer, true); err != nil {
			return ProjectAnswer{}, err
		}
		answer.Tabs = []ProjectTabAnswer{defaultProjectTab()}
	}
	for {
		choices := projectEditChoices(answer)
		selected, err := p.Choose("Projeto "+answer.Name+" — abas na ordem de abertura", choices)
		if err != nil {
			return ProjectAnswer{}, err
		}
		switch {
		case selected == "save":
			if err := validateProjectDraft(answer); err != nil {
				fmt.Fprintln(p.out, err)
				continue
			}
			return answer, nil
		case selected == "cancel":
			return ProjectAnswer{}, huh.ErrUserAborted
		case selected == "identity":
			draft := answer
			err = p.projectIdentity(&draft, creating)
			if err == nil {
				answer = draft
			}
		case selected == "add-tab":
			tab, editErr := p.editProjectTab(ProjectTabAnswer{Hold: true}, &answer, services)
			err = editErr
			if err == nil {
				answer.Tabs = append(answer.Tabs, tab)
			}
		case selected == "add-action":
			action, editErr := p.editProjectAction(ProjectActionAnswer{}, answer, services)
			err = editErr
			if err == nil {
				answer.Actions = append(answer.Actions, action)
			}
		case selected == "tasks":
			var task project.Task
			task, err = p.pickTask(answer.Path, services)
			if err == nil {
				err = services.EditTask(answer.Path, task)
			}
		case strings.HasPrefix(selected, "tab:"):
			index, _ := strconv.Atoi(strings.TrimPrefix(selected, "tab:"))
			err = p.manageProjectTab(&answer, index, services)
		case strings.HasPrefix(selected, "action:"):
			index, _ := strconv.Atoi(strings.TrimPrefix(selected, "action:"))
			err = p.manageProjectAction(&answer, index, services)
		}
		if err != nil && !IsUserAborted(err) {
			fmt.Fprintln(p.out, err)
		}
	}
}

func projectEditChoices(answer ProjectAnswer) []Choice {
	choices := make([]Choice, 0, len(answer.Tabs)+len(answer.Actions)+6)
	for index, tab := range answer.Tabs {
		command := tab.Command
		if tab.Action != "" {
			command = "ação " + tab.Action
		} else if command == "" {
			command = "shell"
		}
		after := "voltar ao shell"
		if !tab.Hold {
			after = "fechar ao sair"
		}
		choices = append(choices, Choice{fmt.Sprintf("tab:%d", index),
			fmt.Sprintf("Aba %d: %s — %s · %s", index+1, tab.Title, command, after)})
	}
	for index, action := range answer.Actions {
		choices = append(choices, Choice{fmt.Sprintf("action:%d", index),
			fmt.Sprintf("Ação: %s → mise run %s", action.Name, action.Task)})
	}
	return append(choices,
		Choice{"add-tab", "+ Adicionar aba"},
		Choice{"add-action", "+ Adicionar ação do mise"},
		Choice{"tasks", "Tarefas do mise — escolher e editar no Neovim (salvas separadamente)"},
		Choice{"identity", "Pasta, shell e preferências do projeto"},
		Choice{"save", "Salvar abas e ações"},
		Choice{"cancel", "Descartar alterações nas abas e ações"},
	)
}

func (p HuhPrompter) projectIdentity(answer *ProjectAnswer, creating bool) error {
	fields := []huh.Field{}
	if creating {
		fields = append(fields, huh.NewInput().Title("Nome curto do projeto").
			Value(&answer.Name).Validate(project.ValidateName))
	}
	fields = append(fields,
		huh.NewInput().Title("Pasta do projeto").Value(&answer.Path).
			Validate(validateRequired("informe a pasta do projeto")),
		huh.NewInput().Title("Shell (opcional)").Description("Vazio usa $SHELL.").Value(&answer.Shell),
		huh.NewConfirm().Title("Manter a aba que abriu o projeto?").
			Affirmative("Manter").Negative("Fechar").Value(&answer.KeepInvokingTab),
	)
	if err := p.projectForm(fields...); err != nil {
		return err
	}
	answer.Name = strings.TrimSpace(answer.Name)
	answer.Path = strings.TrimSpace(answer.Path)
	answer.Shell = strings.TrimSpace(answer.Shell)
	return nil
}

func (p HuhPrompter) manageProjectTab(answer *ProjectAnswer, index int, services ProjectServices) error {
	choices := []Choice{{"edit", "Editar título, comando e comportamento ao sair"}}
	if index > 0 {
		choices = append(choices, Choice{"up", "Mover uma posição para cima"})
	}
	if index+1 < len(answer.Tabs) {
		choices = append(choices, Choice{"down", "Mover uma posição para baixo"})
	}
	if len(answer.Tabs) > 1 {
		choices = append(choices, Choice{"remove", "Remover esta aba"})
	}
	choices = append(choices, Choice{"back", "Voltar"})
	selected, err := p.Choose(answer.Tabs[index].Title, choices)
	if err != nil {
		return err
	}
	switch selected {
	case "edit":
		tab, err := p.editProjectTab(answer.Tabs[index], answer, services)
		if err != nil {
			return err
		}
		answer.Tabs[index] = tab
	case "up":
		answer.Tabs[index-1], answer.Tabs[index] = answer.Tabs[index], answer.Tabs[index-1]
	case "down":
		answer.Tabs[index+1], answer.Tabs[index] = answer.Tabs[index], answer.Tabs[index+1]
	case "remove":
		answer.Tabs = append(answer.Tabs[:index], answer.Tabs[index+1:]...)
	}
	return nil
}

func (p HuhPrompter) editProjectTab(tab ProjectTabAnswer, answer *ProjectAnswer, services ProjectServices) (ProjectTabAnswer, error) {
	kind := "shell"
	if tab.Command != "" {
		kind = "command"
	} else if tab.Action != "" {
		kind = "action"
	}
	if err := p.projectForm(
		huh.NewInput().Title("Título da aba").Value(&tab.Title).
			Validate(validateRequired("o título da aba não pode ser vazio")),
		huh.NewSelect[string]().Title("O que abrir nesta aba?").Value(&kind).Options(
			huh.NewOption("Shell", "shell"), huh.NewOption("Comando direto (Claude, Codex, editor…)", "command"),
			huh.NewOption("Uma ação já cadastrada", "action"), huh.NewOption("Escolher uma tarefa do mise", "task")),
		huh.NewConfirm().Title("Quando o processo terminar").
			Description("Voltar ao shell mantém esta aba na mesma posição.").
			Affirmative("Voltar ao shell").Negative("Fechar a aba").Value(&tab.Hold),
	); err != nil {
		return tab, err
	}
	switch kind {
	case "shell":
		tab.Command, tab.Action = "", ""
	case "command":
		err := p.projectForm(huh.NewInput().Title("Comando").Description("Exemplos: claude, codex ou nvim .").
			Value(&tab.Command).Validate(validateRequired("informe o comando")))
		if err != nil {
			return tab, err
		}
		tab.Action = ""
	case "action":
		if len(answer.Actions) == 0 {
			return tab, fmt.Errorf("adicione uma ação ou escolha uma tarefa do mise")
		}
		choices := make([]Choice, 0, len(answer.Actions))
		for _, action := range answer.Actions {
			choices = append(choices, Choice{action.Name, action.Name + " → " + action.Task})
		}
		action, err := p.chooseValue("Ação desta aba", choices, tab.Action)
		if err != nil {
			return tab, err
		}
		tab.Action, tab.Command = action, ""
	case "task":
		task, err := p.pickTask(answer.Path, services)
		if err != nil {
			return tab, err
		}
		actionName, err := actionForTask(answer, task.Name)
		if err != nil {
			return tab, err
		}
		tab.Action, tab.Command = actionName, ""
	}
	tab.Title, tab.Command = strings.TrimSpace(tab.Title), strings.TrimSpace(tab.Command)
	return tab, nil
}

func actionForTask(answer *ProjectAnswer, task string) (string, error) {
	for _, action := range answer.Actions {
		if action.Task == task {
			return action.Name, nil
		}
	}
	if err := project.ValidateTaskName(task); err != nil {
		return "", err
	}
	base := strings.ReplaceAll(task, "/", ":")
	name := base
	for suffix := 2; validateProjectActionName(answer.Actions)(name) != nil; suffix++ {
		name = fmt.Sprintf("%s-%d", base, suffix)
	}
	answer.Actions = append(answer.Actions, ProjectActionAnswer{Name: name, Task: task})
	return name, nil
}

func (p HuhPrompter) editProjectAction(action ProjectActionAnswer, answer ProjectAnswer, services ProjectServices) (ProjectActionAnswer, error) {
	mode := "choose"
	if action.Task != "" {
		mode = "manual"
	}
	mode, err := p.chooseValue("Tarefa para a ação", []Choice{
		{"choose", "Escolher entre as tarefas disponíveis no mise"},
		{"manual", "Informar ou manter o nome da tarefa"},
	}, mode)
	if err != nil {
		return action, err
	}
	if mode == "choose" {
		task, err := p.pickTask(answer.Path, services)
		if err != nil {
			return action, err
		}
		action.Task = task.Name
		if action.Name == "" {
			action.Name = strings.ReplaceAll(task.Name, "/", ":")
		}
	}
	err = p.projectForm(
		huh.NewInput().Title("Nome curto da ação").Value(&action.Name).Validate(validateProjectActionName(answer.Actions)),
		huh.NewInput().Title("Tarefa do mise").Value(&action.Task).Validate(validateProjectTask),
	)
	action.Name, action.Task = strings.TrimSpace(action.Name), strings.TrimSpace(action.Task)
	return action, err
}

func (p HuhPrompter) manageProjectAction(answer *ProjectAnswer, index int, services ProjectServices) error {
	previous := answer.Actions[index]
	selected, err := p.Choose("Ação "+previous.Name, []Choice{
		{"edit", "Editar nome ou tarefa associada"}, {"source", "Editar a tarefa no Neovim"},
		{"remove", "Remover ação"}, {"back", "Voltar"},
	})
	if err != nil {
		return err
	}
	switch selected {
	case "edit":
		draft := *answer
		draft.Actions = append(append([]ProjectActionAnswer(nil), answer.Actions[:index]...), answer.Actions[index+1:]...)
		action, err := p.editProjectAction(previous, draft, services)
		if err != nil {
			return err
		}
		answer.Actions[index] = action
		for i := range answer.Tabs {
			if answer.Tabs[i].Action == previous.Name {
				answer.Tabs[i].Action = action.Name
			}
		}
	case "source":
		tasks, err := services.Tasks(answer.Path)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			if task.Matches(previous.Task) {
				return services.EditTask(answer.Path, task)
			}
		}
		return fmt.Errorf("a tarefa %q não foi encontrada pelo mise", previous.Task)
	case "remove":
		for _, tab := range answer.Tabs {
			if tab.Action == previous.Name {
				return fmt.Errorf("a aba %q usa esta ação; altere a aba antes de remover a ação", tab.Title)
			}
		}
		answer.Actions = append(answer.Actions[:index], answer.Actions[index+1:]...)
	}
	return nil
}

func (p HuhPrompter) pickTask(dir string, services ProjectServices) (project.Task, error) {
	tasks, err := services.Tasks(dir)
	if err != nil {
		return project.Task{}, err
	}
	if len(tasks) == 0 {
		return project.Task{}, fmt.Errorf("nenhuma tarefa do mise encontrada em %s", dir)
	}
	choices := TaskChoices(tasks)
	selected, err := p.Choose("Tarefas do mise", choices)
	if err != nil {
		return project.Task{}, err
	}
	for _, task := range tasks {
		if task.Name == selected {
			return task, nil
		}
	}
	return project.Task{}, fmt.Errorf("tarefa desconhecida: %s", selected)
}

func TaskChoices(tasks []project.Task) []Choice {
	choices := make([]Choice, 0, len(tasks))
	for _, task := range tasks {
		scope := "projeto"
		if task.Global {
			scope = "global"
		}
		choices = append(choices, Choice{task.Name,
			fmt.Sprintf("%s — %s [%s] · %s", task.Name, task.Description, scope, task.EditPath())})
	}
	return choices
}

func validateProjectDraft(answer ProjectAnswer) error {
	manifest := project.Manifest{Version: 2, Path: answer.Path, Shell: answer.Shell, Actions: map[string]project.Action{}}
	for _, action := range answer.Actions {
		manifest.Actions[action.Name] = project.Action{Task: action.Task}
	}
	for _, tab := range answer.Tabs {
		manifest.Tabs = append(manifest.Tabs, project.Tab{Title: tab.Title, Command: tab.Command, Action: tab.Action})
	}
	return project.Validate(manifest)
}
