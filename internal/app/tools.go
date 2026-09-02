package app

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
	"github.com/roqem/konen/internal/ui"
)

type toolState struct {
	Tools map[string]any `toml:"tools"`
}

func (a *App) runTool(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add")
	}
	if args[0] != "add" {
		return fmt.Errorf("ação de ferramenta desconhecida: %s", args[0])
	}
	return a.runToolAdd(ctx, args[1:])
}

func (a *App) runToolAdd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("konen tool add", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	yes := flags.Bool("yes", false, "grava sem pedir confirmação")
	dryRun := flags.Bool("dry-run", false, "mostra a alteração sem gravar")
	if help, err := parseCommandFlags(flags, args); err != nil {
		return err
	} else if help {
		return nil
	}
	if flags.NArg() > 2 {
		return errors.New("uso: konen tool add [--dry-run] [--yes] NOME [VERSÃO]")
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
	answer := ui.ToolAnswer{Version: "latest"}
	if flags.NArg() >= 1 {
		answer.Name = flags.Arg(0)
	}
	if flags.NArg() == 2 {
		answer.Version = flags.Arg(1)
	}
	if answer.Name == "" {
		if !a.options.Interactive {
			return errors.New("informe a ferramenta: konen tool add NOME [VERSÃO]")
		}
		answer, err = a.options.Prompter.Tool(answer)
		if err != nil {
			return err
		}
	}
	answer.Name = strings.TrimSpace(answer.Name)
	answer.Version = strings.TrimSpace(answer.Version)
	if answer.Version == "" {
		answer.Version = "latest"
	}
	if err := validateToolAnswer(answer); err != nil {
		return err
	}

	configPath := filepath.Join(stateDir, "mise.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var current toolState
	if err := toml.Unmarshal(before, &current); err != nil {
		return fmt.Errorf("mise.toml inválido: %w", err)
	}
	if version, ok := current.Tools[answer.Name].(string); ok && version == answer.Version {
		fmt.Fprintf(a.options.Out, "Ferramenta já declarada: %s@%s\n", answer.Name, answer.Version)
		return nil
	}

	after, err := a.previewToolConfig(ctx, stateDir, misePath, configPath, before, answer)
	if err != nil {
		return err
	}
	if bytes.Equal(before, after) {
		fmt.Fprintf(a.options.Out, "Ferramenta já declarada: %s@%s\n", answer.Name, answer.Version)
		return nil
	}

	fmt.Fprintf(a.options.Out, "Alteração proposta em %s:\n", configPath)
	fmt.Fprint(a.options.Out, ui.RenderDiff("mise.toml", string(before), string(after)))
	if *dryRun {
		fmt.Fprintln(a.options.Out, "Nenhuma alteração foi gravada.")
		return nil
	}
	if !*yes {
		confirmed, err := a.options.Prompter.Confirm("Gravar esta ferramenta no estado?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(a.options.Out, "Alteração cancelada.")
			return nil
		}
	}

	currentBytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, currentBytes) {
		return errors.New("mise.toml mudou durante a revisão; execute o comando novamente")
	}
	writeEnvironment := append(miseStateEnvironment(stateDir), "MISE_AUTO_INSTALL=false")
	if err := a.options.Runner.RunEnv(ctx, stateDir, writeEnvironment, misePath,
		toolConfigSetArgs(configPath, answer)...); err != nil {
		return fmt.Errorf("mise config set: %w", err)
	}
	if err := a.options.Runner.RunEnv(
		ctx, stateDir, miseStateEnvironment(stateDir), misePath, "trust", configPath,
	); err != nil {
		return fmt.Errorf("ferramenta gravada, mas não foi possível confiar no mise.toml: %w", err)
	}
	if _, err := a.stateTrust().Trust(stateDir); err != nil {
		return fmt.Errorf("ferramenta gravada, mas a aprovação local não pôde ser atualizada: %w", err)
	}

	fmt.Fprintf(a.options.Out, "Ferramenta adicionada ao estado: %s@%s\n", answer.Name, answer.Version)
	fmt.Fprintln(a.options.Out, "Próximo passo: execute `konen plan`.")
	return nil
}

func (a *App) previewToolConfig(
	ctx context.Context,
	stateDir, misePath, configPath string,
	before []byte,
	answer ui.ToolAnswer,
) ([]byte, error) {
	temporaryDir, err := os.MkdirTemp("", "konen-tool-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temporaryDir)
	temporaryConfig := filepath.Join(temporaryDir, "mise.toml")
	if err := os.WriteFile(temporaryConfig, before, 0o600); err != nil {
		return nil, err
	}
	environment := append(miseStateEnvironment(temporaryDir),
		"MISE_AUTO_INSTALL=false",
		"MISE_TRUSTED_CONFIG_PATHS="+temporaryDir,
	)
	if err := a.options.Runner.RunEnv(ctx, temporaryDir, environment, misePath,
		toolConfigSetArgs(temporaryConfig, answer)...); err != nil {
		return nil, fmt.Errorf("o mise recusou a ferramenta ou a versão: %w", err)
	}
	after, err := os.ReadFile(temporaryConfig)
	if err != nil {
		return nil, err
	}
	return after, nil
}

func toolConfigSetArgs(configPath string, answer ui.ToolAnswer) []string {
	return []string{
		"config", "set",
		"--file", configPath,
		"--type", "string",
		"--", "tools." + answer.Name, answer.Version,
	}
}

func validateToolAnswer(answer ui.ToolAnswer) error {
	if answer.Name == "" {
		return errors.New("o nome da ferramenta não pode ser vazio")
	}
	if answer.Version == "" {
		return errors.New("a versão não pode ser vazia")
	}
	if strings.Contains(answer.Name, ".") {
		return errors.New("nomes com ponto ainda não são suportados pelo assistente; edite [tools] no mise.toml")
	}
	if strings.HasPrefix(answer.Name, "-") || strings.HasPrefix(answer.Version, "-") {
		return errors.New("ferramenta e versão não podem começar com hífen")
	}
	for _, value := range []string{answer.Name, answer.Version} {
		for _, character := range value {
			if unicode.IsSpace(character) || unicode.IsControl(character) {
				return errors.New("ferramenta e versão não podem conter espaços ou caracteres de controle")
			}
		}
	}
	return nil
}
