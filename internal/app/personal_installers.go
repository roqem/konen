package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

func (a *App) runPersonalInstaller(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add")
	}
	if args[0] != "add" {
		return fmt.Errorf("ação de instalador desconhecida: %s", args[0])
	}
	return a.runPersonalInstallerAdd(ctx, args[1:])
}

func (a *App) runPersonalInstallerAdd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("konen installer add", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	yes := flags.Bool("yes", false, "grava sem pedir confirmação")
	dryRun := flags.Bool("dry-run", false, "mostra os arquivos sem gravar")
	source := flags.String("from", "", "importa o conteúdo de um arquivo existente")
	if help, err := parseCommandFlags(flags, args); err != nil {
		return err
	} else if help {
		return nil
	}
	if flags.NArg() > 1 {
		return errors.New("uso: konen installer add [--from ARQUIVO] [--dry-run] [--yes] [NOME]")
	}
	if *dryRun && *yes {
		return errors.New("use apenas --dry-run ou --yes")
	}
	if !a.options.Interactive && !*dryRun && !*yes {
		return errors.New("em modo não interativo, revise com `--dry-run` ou confirme a gravação com `--yes`")
	}

	stateDir, misePath, err := a.loadTrustedMise(ctx)
	if err != nil {
		return err
	}
	answer := ui.PersonalInstallerAnswer{Mode: "create", Source: strings.TrimSpace(*source)}
	if answer.Source != "" {
		answer.Mode = "import"
	}
	if flags.NArg() == 1 {
		answer.Name = flags.Arg(0)
	}
	if answer.Name == "" && answer.Source != "" {
		answer.Name = filepath.Base(filepath.Clean(answer.Source))
	}
	if answer.Name == "" {
		if !a.options.Interactive {
			return errors.New("informe o nome: konen installer add NOME ou konen installer add --from ARQUIVO [NOME]")
		}
		answer, err = a.options.Prompter.PersonalInstaller(answer)
		if err != nil {
			return err
		}
	}
	answer.Mode = strings.TrimSpace(answer.Mode)
	answer.Name = strings.TrimSpace(answer.Name)
	answer.Source = strings.TrimSpace(answer.Source)
	if answer.Name == "" && answer.Source != "" {
		answer.Name = filepath.Base(filepath.Clean(answer.Source))
	}
	if err := validatePersonalInstallerAnswer(answer); err != nil {
		return err
	}

	var contents []byte
	sourceDescription := "novo esqueleto sem comandos de instalação"
	if answer.Mode == "import" {
		sourcePath, err := a.resolveDotfileTarget(answer.Source)
		if err != nil {
			return err
		}
		contents, err = readReviewedExecutable(sourcePath, "instalador")
		if err != nil {
			return err
		}
		sourceDescription = sourcePath
	} else {
		contents = personalInstallerScaffold(answer.Name)
	}

	installerRelative := filepath.ToSlash(filepath.Join("mise-tasks", "install", answer.Name))
	installerPath := filepath.Join(stateDir, filepath.FromSlash(installerRelative))
	if err := executableDestinationAvailable(installerPath, "instalador pessoal"); err != nil {
		return err
	}
	if err := validatePersonalInstallerParents(stateDir); err != nil {
		return err
	}

	configPath := filepath.Join(stateDir, "mise.toml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	taskName := "install:" + answer.Name
	afterConfig, alreadySelected, err := state.AddTaskRunReference(beforeConfig, taskName)
	if err != nil {
		return fmt.Errorf("não foi possível incluir o instalador no fluxo de aplicação: %w", err)
	}
	configChanged := !bytes.Equal(beforeConfig, afterConfig)

	fmt.Fprintf(a.options.Out, "Instalador pessoal: %s\n", answer.Name)
	fmt.Fprintf(a.options.Out, "Tarefa do mise: %s\n", taskName)
	fmt.Fprintf(a.options.Out, "Origem: %s\n", sourceDescription)
	fmt.Fprintf(a.options.Out, "Destino: %s\n", installerPath)
	fmt.Fprintln(a.options.Out, "Permissão: executável (0755)")
	fmt.Fprintln(a.options.Out, "Arquivo proposto:")
	fmt.Fprint(a.options.Out, ui.RenderDiff(installerRelative, "", string(contents)))
	if configChanged {
		fmt.Fprintln(a.options.Out, "Inclusão proposta no fluxo de aplicação:")
		fmt.Fprint(a.options.Out, ui.RenderDiff("mise.toml", string(beforeConfig), string(afterConfig)))
	} else if alreadySelected {
		fmt.Fprintf(a.options.Out, "Aplicação: %s já estava incluído.\n", taskName)
	}
	if answer.Mode == "create" {
		fmt.Fprintln(a.options.Out, "Atenção: o esqueleto termina com erro até ser implementado; assim `konen apply` não pode indicar uma instalação inexistente.")
	}
	if *dryRun {
		fmt.Fprintln(a.options.Out, "Nenhum arquivo foi gravado e nenhuma tarefa foi executada.")
		return nil
	}
	if !*yes {
		confirmed, err := a.options.Prompter.Confirm("Adicionar e selecionar este instalador pessoal?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(a.options.Out, "Alteração cancelada.")
			return nil
		}
	}

	currentConfig, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(beforeConfig, currentConfig) {
		return errors.New("mise.toml mudou durante a revisão; execute o comando novamente")
	}
	if err := executableDestinationAvailable(installerPath, "instalador pessoal"); err != nil {
		return err
	}
	if err := createPersonalInstaller(stateDir, installerPath, contents); err != nil {
		return err
	}
	if configChanged {
		if err := replaceFile(configPath, afterConfig); err != nil {
			_ = os.Remove(installerPath)
			return err
		}
		if err := a.options.Runner.RunEnv(
			ctx, stateDir, miseStateEnvironment(stateDir), misePath, "trust", configPath,
		); err != nil {
			return fmt.Errorf("instalador gravado, mas não foi possível confiar no mise.toml: %w", err)
		}
	}
	if _, err := a.stateTrust().Trust(stateDir); err != nil {
		return fmt.Errorf("instalador gravado, mas a aprovação local não pôde ser atualizada: %w", err)
	}

	fmt.Fprintf(a.options.Out, "Instalador pessoal adicionado: %s\n", installerRelative)
	fmt.Fprintf(a.options.Out, "`konen apply` agora executará %s.\n", taskName)
	if answer.Mode == "create" {
		fmt.Fprintf(a.options.Out, "Implemente %s e execute `konen trust` antes do próximo plano.\n", installerPath)
	}
	fmt.Fprintln(a.options.Out, "O instalador não foi executado durante o cadastro.")
	return nil
}

func validatePersonalInstallerAnswer(answer ui.PersonalInstallerAnswer) error {
	if answer.Mode != "create" && answer.Mode != "import" {
		return errors.New("modo inválido; use create ou import")
	}
	if err := validateExecutableName(answer.Name); err != nil {
		return err
	}
	if answer.Mode == "import" && answer.Source == "" {
		return errors.New("informe o arquivo que deve ser importado")
	}
	return nil
}

func personalInstallerScaffold(name string) []byte {
	return []byte(fmt.Sprintf(`#!/bin/sh
#MISE description="Instalador pessoal %s (não implementado)"
set -eu

printf '%%s\n' '%s ainda não foi implementado.' >&2
exit 1
`, name, name))
}

func validatePersonalInstallerParents(stateDir string) error {
	for _, relative := range []string{"mise-tasks", filepath.Join("mise-tasks", "install")} {
		path := filepath.Join(stateDir, relative)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("a pasta de instaladores pessoais deve ser um diretório real: %s", path)
		}
	}
	return nil
}

func createPersonalInstaller(stateDir, path string, contents []byte) error {
	for _, relative := range []string{"mise-tasks", filepath.Join("mise-tasks", "install")} {
		directory := filepath.Join(stateDir, relative)
		if err := os.Mkdir(directory, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := os.Lstat(directory)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("a pasta de instaladores pessoais deve ser um diretório real: %s", directory)
		}
	}
	return createExclusiveExecutable(path, contents, "instalador pessoal")
}
