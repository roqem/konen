package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/ui"
)

func (a *App) runProjectMenu(ctx context.Context, store project.Store) error {
	if !a.options.Interactive {
		return errors.New("use `konen projects` para listar ou `konen project --help` para consultar os comandos")
	}
	for {
		projects, err := store.List()
		if err != nil {
			return err
		}
		choices := make([]ui.Choice, 0, len(projects)+2)
		for _, item := range projects {
			choices = append(choices, ui.Choice{Value: item.Name, Label: fmt.Sprintf("%s — %d abas · %s", item.Name, len(item.Manifest.Tabs), item.Manifest.Path)})
		}
		choices = append(choices, ui.Choice{Value: "+", Label: "+ Cadastrar projeto"}, ui.Choice{Value: "", Label: "Sair"})
		name, err := a.options.Prompter.Choose("Projetos — abrir e gerenciar", choices)
		if err != nil || name == "" {
			return err
		}
		if name == "+" {
			err = a.runProjectAdd(ctx, store, nil)
		} else {
			err = a.manageProject(ctx, store, name)
		}
		if err != nil && !ui.IsUserAborted(err) {
			return err
		}
	}
}

func (a *App) manageProject(ctx context.Context, store project.Store, name string) error {
	for {
		selected, err := a.options.Prompter.Choose("Projeto "+name, []ui.Choice{
			{Value: "open", Label: "Abrir a sessão completa"},
			{Value: "tab", Label: "Abrir uma aba"},
			{Value: "edit", Label: "Editar e ordenar abas e ações"},
			{Value: "run", Label: "Executar uma ação"},
			{Value: "tasks", Label: "Ver tarefas do mise"},
			{Value: "task-edit", Label: "Editar uma tarefa no Neovim"},
			{Value: "preview", Label: "Ver a sessão configurada"},
			{Value: "back", Label: "Voltar aos projetos"},
		})
		if err != nil {
			return err
		}
		switch selected {
		case "back":
			return nil
		case "open":
			err = a.runDev(ctx, []string{name})
		case "tab":
			var manifest project.Manifest
			manifest, _, err = store.Load(name)
			if err == nil {
				choices := make([]ui.Choice, 0, len(manifest.Tabs))
				for _, tab := range manifest.Tabs {
					choices = append(choices, ui.Choice{Value: tab.Title, Label: tab.Title + " — " + projectTabDescription(manifest, tab)})
				}
				var title string
				title, err = a.options.Prompter.Choose("Abrir aba de "+name, choices)
				if err == nil {
					err = a.runDev(ctx, []string{name, "--tab", title})
				}
			}
		case "edit":
			err = a.runProjectEdit(ctx, store, []string{name})
		case "run":
			var manifest project.Manifest
			manifest, _, err = store.Load(name)
			if err == nil {
				var action string
				action, err = a.options.Prompter.ChooseProjectAction(name, sortedActionNames(manifest))
				if err == nil {
					err = a.runNamedProjectAction(ctx, []string{name, action}, true)
				}
			}
		case "tasks":
			err = a.runProjectTasks(ctx, store, []string{name})
		case "task-edit":
			err = a.runProjectTaskEdit(ctx, store, []string{name})
		case "preview":
			err = a.runDev(ctx, []string{name, "--dry-run"})
		}
		if err != nil && !ui.IsUserAborted(err) {
			fmt.Fprintln(a.options.Out, err)
		}
	}
}
