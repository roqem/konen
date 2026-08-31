package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

func (a *App) runRepo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add")
	}
	if args[0] != "add" {
		return fmt.Errorf("ação de repositório desconhecida: %s", args[0])
	}
	return a.runRepoAdd(ctx, args[1:])
}

func (a *App) runRepoAdd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("repo add", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	yes := flags.Bool("yes", false, "grava sem pedir confirmação")
	dryRun := flags.Bool("dry-run", false, "mostra a alteração sem gravar")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 3 {
		return errors.New("uso: konen repo add [--dry-run] DESTINO URL [REF]")
	}
	if !a.options.Interactive && !*dryRun && !*yes {
		return errors.New("em modo não interativo, revise com `--dry-run` ou confirme a gravação com `--yes`")
	}

	stateDir, misePath, err := a.loadTrustedMise(ctx)
	if err != nil {
		return err
	}
	answer := ui.RepositoryAnswer{}
	if flags.NArg() >= 1 {
		answer.Destination = flags.Arg(0)
	}
	if flags.NArg() >= 2 {
		answer.URL = flags.Arg(1)
	}
	if flags.NArg() == 3 {
		answer.Ref = flags.Arg(2)
	}
	if answer.Destination == "" || answer.URL == "" {
		if !a.options.Interactive {
			return errors.New("informe destino e URL: konen repo add DESTINO URL [REF]")
		}
		answer, err = a.options.Prompter.Repository(answer)
		if err != nil {
			return err
		}
	}
	answer.Destination, err = normalizeRepositoryDestination(answer.Destination, a.options.HomeDir)
	if err != nil {
		return err
	}
	answer.URL = strings.TrimSpace(answer.URL)
	answer.Ref = strings.TrimSpace(answer.Ref)
	if err := validateRepositoryAnswer(answer); err != nil {
		return err
	}

	configPath := filepath.Join(stateDir, "mise.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	literal, err := repositoryLiteral(answer)
	if err != nil {
		return err
	}
	after, exists, err := state.AddTableEntry(
		before, []string{"bootstrap", "repos"}, answer.Destination, literal,
	)
	if err != nil {
		return fmt.Errorf("mise.toml inválido: %w", err)
	}
	if exists {
		fmt.Fprintf(a.options.Out, "Repositório já declarado: %s\n", answer.Destination)
		return nil
	}

	fmt.Fprintf(a.options.Out, "Destino: %s\n", answer.Destination)
	fmt.Fprintf(a.options.Out, "Origem Git: %s\n", answer.URL)
	if answer.Ref == "" {
		fmt.Fprintln(a.options.Out, "Referência: não fixada; um checkout existente não será atualizado pelo apply")
	} else {
		fmt.Fprintf(a.options.Out, "Referência: %s\n", answer.Ref)
	}
	written, err := a.reviewStateConfigChange(
		ctx, stateDir, misePath, before, after, *dryRun, *yes,
		"Gravar este repositório no estado?",
	)
	if err != nil || !written {
		return err
	}
	fmt.Fprintf(a.options.Out, "Repositório adicionado ao estado: %s\n", answer.Destination)
	fmt.Fprintln(a.options.Out, "Nenhum clone foi feito. Próximo passo: execute `konen plan --only repos`.")
	return nil
}

func normalizeRepositoryDestination(input, homeDir string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" || input == "~" {
		return "", errors.New("informe uma pasta de destino, não a raiz do diretório pessoal")
	}
	if strings.HasPrefix(input, "~/") {
		relative := filepath.Clean(strings.TrimPrefix(input, "~/"))
		if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", errors.New("o destino não pode escapar do diretório pessoal")
		}
		return "~/" + filepath.ToSlash(relative), nil
	}
	if !filepath.IsAbs(input) {
		return "", errors.New("o destino deve ser absoluto ou começar com ~/")
	}
	clean := filepath.Clean(input)
	relative, err := filepath.Rel(filepath.Clean(homeDir), clean)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(relative), nil
	}
	return clean, nil
}

func validateRepositoryAnswer(answer ui.RepositoryAnswer) error {
	if answer.URL == "" {
		return errors.New("a URL do repositório não pode ser vazia")
	}
	for _, value := range []string{answer.URL, answer.Ref} {
		if strings.HasPrefix(value, "-") {
			return errors.New("URL e referência não podem começar com hífen")
		}
		for _, character := range value {
			if unicode.IsSpace(character) || unicode.IsControl(character) {
				return errors.New("URL e referência não podem conter espaços ou caracteres de controle")
			}
		}
	}
	return nil
}

func repositoryLiteral(answer ui.RepositoryAnswer) (string, error) {
	url, err := state.TOMLString(answer.URL)
	if err != nil {
		return "", err
	}
	literal := "{ url = " + url
	if answer.Ref != "" {
		ref, err := state.TOMLString(answer.Ref)
		if err != nil {
			return "", err
		}
		literal += ", ref = " + ref
	}
	return literal + " }", nil
}
