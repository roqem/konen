package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/ui"
)

func (a *App) projectServices(ctx context.Context) ui.ProjectServices {
	return ui.ProjectServices{
		Tasks:    func(dir string) ([]project.Task, error) { return a.projectTasks(ctx, dir) },
		EditTask: func(dir string, task project.Task) error { return a.editProjectTask(ctx, dir, task) },
	}
}

func (a *App) projectTasks(ctx context.Context, dir string) ([]project.Task, error) {
	resolved, err := project.ResolvePath(dir, a.options.HomeDir)
	if err != nil {
		return nil, err
	}
	mise, err := a.findCommand("mise")
	if err != nil {
		return nil, errors.New("mise não foi encontrado; execute `konen doctor`")
	}
	output, err := a.options.Runner.OutputEnv(ctx, resolved, []string{"MISE_AUTO_INSTALL=0"}, mise, "tasks", "ls", "--json")
	if err != nil {
		return nil, fmt.Errorf("não foi possível listar as tarefas em %s; revise a configuração e a confiança do mise: %w", resolved, err)
	}
	return project.DecodeTasks([]byte(output))
}

func (a *App) projectForArgs(store project.Store, args []string) (string, project.Manifest, string, error) {
	if len(args) > 1 {
		return "", project.Manifest{}, "", errors.New("informe no máximo um nome de projeto")
	}
	var name string
	var err error
	if len(args) == 1 {
		name = args[0]
	} else {
		name, err = a.selectProject(store)
		if err != nil {
			return "", project.Manifest{}, "", err
		}
	}
	manifest, _, err := store.Load(name)
	if err != nil {
		return "", project.Manifest{}, "", err
	}
	dir, err := store.ResolveProjectPath(manifest)
	return name, manifest, dir, err
}

func (a *App) runProjectTasks(ctx context.Context, store project.Store, args []string) error {
	name, _, dir, err := a.projectForArgs(store, args)
	if err != nil {
		return err
	}
	tasks, err := a.projectTasks(ctx, dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.options.Out, "Tarefas disponíveis em %s (%s):\n", name, dir)
	if len(tasks) == 0 {
		fmt.Fprintln(a.options.Out, "Nenhuma tarefa encontrada.")
		return nil
	}
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		scope := "projeto"
		if task.Global {
			scope = "global"
		}
		rows = append(rows, []string{task.Name, task.Description, task.Command(), scope, task.EditPath()})
	}
	fmt.Fprint(a.options.Out, ui.RenderTable([]string{"Tarefa", "Descrição", "Comando", "Escopo", "Arquivo"}, rows))
	fmt.Fprintf(a.options.Out, "Editar no Neovim: konen project task edit %s [TAREFA]\n", name)
	return nil
}

func (a *App) runProjectTaskEdit(ctx context.Context, store project.Store, args []string) error {
	if len(args) == 1 && isHelpArgument(args[0]) {
		a.printCommandGroup("Uso", [][2]string{{"konen project task edit [NOME] [TAREFA]", "seleciona e edita uma tarefa no Neovim"}})
		return nil
	}
	if len(args) > 2 {
		return errors.New("uso: konen project task edit [NOME] [TAREFA]")
	}
	if !a.options.Interactive {
		return errors.New("editar tarefas no Neovim requer uma sessão interativa")
	}
	_, _, dir, err := a.projectForArgs(store, args[:min(len(args), 1)])
	if err != nil {
		return err
	}
	tasks, err := a.projectTasks(ctx, dir)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return errors.New("nenhuma tarefa do mise encontrada neste projeto")
	}
	var selected string
	if len(args) == 2 {
		selected = args[1]
	} else {
		selected, err = a.options.Prompter.Choose("Editar tarefa no Neovim", ui.TaskChoices(tasks))
		if err != nil {
			return err
		}
	}
	for _, task := range tasks {
		if task.Matches(selected) {
			return a.editProjectTask(ctx, dir, task)
		}
	}
	return fmt.Errorf("a tarefa %q não foi encontrada pelo mise", selected)
}

func (a *App) editProjectTask(ctx context.Context, dir string, task project.Task) error {
	nvim, err := a.findCommand("nvim")
	if err != nil {
		return errors.New("Neovim não foi encontrado; instale-o para editar tarefas")
	}
	dir, err = project.ResolvePath(dir, a.options.HomeDir)
	if err != nil {
		return err
	}
	source := task.EditPath()
	if source == "" {
		return fmt.Errorf("mise não informou o arquivo da tarefa %q", task.Name)
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(dir, source)
	}
	path, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("a tarefa não está em um arquivo regular: %s", path)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !utf8.Valid(before) || bytes.ContainsRune(before, '\x00') {
		return errors.New("a edição assistida requer um arquivo de texto UTF-8")
	}
	// A private draft keeps invalid edits and cancellation away from live tasks.
	tempDir, err := os.MkdirTemp("", "konen-task-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	draft := filepath.Join(tempDir, filepath.Base(source))
	if err := os.WriteFile(draft, before, 0o600); err != nil {
		return err
	}
	line := 1
	isTOML := task.File == ""
	if isTOML {
		line = project.TaskLine(before, task.Name)
	}
	fmt.Fprintf(a.options.Out, "Tarefa: %s\nArquivo: %s\nComando: %s\nEdite o rascunho no Neovim (:wq para revisar; :cq para cancelar).\n", task.Name, source, task.Command())
	for {
		if err := a.options.Runner.Run(ctx, dir, nvim, "+"+strconv.Itoa(line), "--", draft); err != nil {
			return fmt.Errorf("edição interrompida; o arquivo original foi preservado: %w", err)
		}
		after, err := os.ReadFile(draft)
		if err != nil {
			return err
		}
		if bytes.Equal(before, after) {
			fmt.Fprintln(a.options.Out, "Nenhuma alteração na tarefa.")
			return nil
		}
		var validation error
		if !utf8.Valid(after) || bytes.ContainsRune(after, '\x00') {
			validation = errors.New("o arquivo deve continuar sendo texto UTF-8")
		} else if isTOML {
			var document map[string]any
			validation = toml.Unmarshal(after, &document)
		}
		if validation != nil {
			fmt.Fprintf(a.options.Out, "Não foi possível validar o arquivo: %v\n", validation)
			retry, err := a.options.Prompter.Confirm("Voltar ao Neovim para corrigir?")
			if err != nil {
				return err
			}
			if retry {
				continue
			}
			fmt.Fprintln(a.options.Out, "Edição cancelada; arquivo original preservado.")
			return nil
		}
		fmt.Fprint(a.options.Out, ui.RenderDiff(source, string(before), string(after)))
		confirmed, err := a.options.Prompter.Confirm("Salvar esta alteração na tarefa do mise?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(a.options.Out, "Edição cancelada; arquivo original preservado.")
			return nil
		}
		currentPath, err := filepath.EvalSymlinks(source)
		if err != nil || currentPath != path {
			return errors.New("o caminho da tarefa mudou durante a edição; nenhuma alteração foi gravada")
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		currentInfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(before, current) || currentInfo.Mode() != info.Mode() {
			return errors.New("a tarefa mudou durante a edição; nenhuma alteração foi gravada")
		}
		if err := replaceFile(path, after); err != nil {
			return err
		}
		fmt.Fprintf(a.options.Out, "Tarefa atualizada: %s\nNenhuma tarefa foi executada. Revise a confiança do mise antes de executar; tarefas globais do estado também podem exigir `konen trust`.\n", source)
		return nil
	}
}
