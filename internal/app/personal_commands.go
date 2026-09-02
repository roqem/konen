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
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

const (
	personalCommandPathExpression = "{{ config_source | canonicalize | dirname }}/scripts/bin"
	maxReviewedExecutableBytes    = 1024 * 1024
)

func (a *App) runPersonalCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add")
	}
	if args[0] != "add" {
		return fmt.Errorf("ação de comando desconhecida: %s", args[0])
	}
	return a.runPersonalCommandAdd(ctx, args[1:])
}

func (a *App) runPersonalCommandAdd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("konen command add", flag.ContinueOnError)
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
		return errors.New("uso: konen command add [--from ARQUIVO] [--dry-run] [--yes] [NOME]")
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
	answer := ui.PersonalCommandAnswer{Mode: "create", Source: strings.TrimSpace(*source)}
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
			return errors.New("informe o nome: konen command add NOME ou konen command add --from ARQUIVO [NOME]")
		}
		answer, err = a.options.Prompter.PersonalCommand(answer)
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
	if err := validatePersonalCommandAnswer(answer); err != nil {
		return err
	}

	var contents []byte
	sourceDescription := "novo esqueleto seguro"
	if answer.Mode == "import" {
		sourcePath, err := a.resolveDotfileTarget(answer.Source)
		if err != nil {
			return err
		}
		contents, err = readReviewedExecutable(sourcePath, "comando")
		if err != nil {
			return err
		}
		sourceDescription = sourcePath
	} else {
		contents = personalCommandScaffold(answer.Name)
	}

	commandRelative := filepath.ToSlash(filepath.Join("scripts", "bin", answer.Name))
	commandPath := filepath.Join(stateDir, filepath.FromSlash(commandRelative))
	if err := executableDestinationAvailable(commandPath, "comando pessoal"); err != nil {
		return err
	}
	if err := validatePersonalCommandParents(stateDir); err != nil {
		return err
	}

	configPath := filepath.Join(stateDir, "mise.toml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	afterConfig, configChanged, err := ensurePersonalCommandsOnPath(beforeConfig)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.options.Out, "Comando pessoal: %s\n", answer.Name)
	fmt.Fprintf(a.options.Out, "Origem: %s\n", sourceDescription)
	fmt.Fprintf(a.options.Out, "Destino: %s\n", commandPath)
	fmt.Fprintln(a.options.Out, "Permissão: executável (0755)")
	fmt.Fprintln(a.options.Out, "Arquivo proposto:")
	fmt.Fprint(a.options.Out, ui.RenderDiff(commandRelative, "", string(contents)))
	if configChanged {
		fmt.Fprintln(a.options.Out, "O estado também precisa expor scripts/bin no PATH:")
		fmt.Fprint(a.options.Out, ui.RenderDiff("mise.toml", string(beforeConfig), string(afterConfig)))
	}
	if *dryRun {
		fmt.Fprintln(a.options.Out, "Nenhum arquivo foi gravado.")
		return nil
	}
	if !*yes {
		confirmed, err := a.options.Prompter.Confirm("Adicionar este comando pessoal ao estado?")
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
	if err := executableDestinationAvailable(commandPath, "comando pessoal"); err != nil {
		return err
	}
	if err := createPersonalCommand(stateDir, commandPath, contents); err != nil {
		return err
	}
	if configChanged {
		if err := replaceFile(configPath, afterConfig); err != nil {
			_ = os.Remove(commandPath)
			return err
		}
		if err := a.options.Runner.RunEnv(
			ctx, stateDir, miseStateEnvironment(stateDir), misePath, "trust", configPath,
		); err != nil {
			return fmt.Errorf("comando gravado, mas não foi possível confiar no mise.toml: %w", err)
		}
	}
	if _, err := a.stateTrust().Trust(stateDir); err != nil {
		return fmt.Errorf("comando gravado, mas a aprovação local não pôde ser atualizada: %w", err)
	}

	fmt.Fprintf(a.options.Out, "Comando pessoal adicionado: %s\n", commandRelative)
	if answer.Mode == "create" {
		fmt.Fprintf(a.options.Out, "Edite %s para implementar o comando.\n", commandPath)
	}
	fmt.Fprintln(a.options.Out, "O comando não foi executado. Confira-o com `konen status` e versione o estado quando estiver pronto.")
	return nil
}

func validatePersonalCommandAnswer(answer ui.PersonalCommandAnswer) error {
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

func validateExecutableName(name string) error {
	if name == "" {
		return errors.New("o nome não pode ser vazio")
	}
	if len(name) > 128 {
		return errors.New("o nome não pode exceder 128 caracteres")
	}
	for index, character := range name {
		valid := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if index > 0 && (character == '-' || character == '_' || character == '.') {
			valid = true
		}
		if !valid {
			return errors.New("o nome deve começar com letra ou número e usar apenas letras, números, ponto, hífen ou sublinhado")
		}
	}
	return nil
}

func personalCommandScaffold(name string) []byte {
	return []byte(fmt.Sprintf(`#!/bin/sh
set -eu

printf '%%s\n' '%s ainda não foi implementado.' >&2
exit 1
`, name))
}

func readReviewedExecutable(path, subject string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler o %s de origem: %w", subject, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("o %s de origem não pode ser um link simbólico; informe o arquivo real", subject)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("o %s de origem deve ser um arquivo regular", subject)
	}
	if info.Size() > maxReviewedExecutableBytes {
		return nil, fmt.Errorf("o %s de origem excede o limite de %d bytes", subject, maxReviewedExecutableBytes)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("o %s de origem está vazio", subject)
	}
	if len(contents) > maxReviewedExecutableBytes {
		return nil, fmt.Errorf("o %s de origem excede o limite de %d bytes", subject, maxReviewedExecutableBytes)
	}
	if !utf8.Valid(contents) {
		return nil, fmt.Errorf("o %s de origem precisa ser um arquivo de texto UTF-8", subject)
	}
	for _, character := range string(contents) {
		if character < 0x20 && character != '\n' && character != '\t' || character == 0x7f {
			return nil, fmt.Errorf("o %s de origem contém caracteres de controle que não podem ser exibidos com segurança", subject)
		}
	}
	if !bytes.HasPrefix(contents, []byte("#!")) {
		return nil, fmt.Errorf("o %s de origem precisa começar com um shebang, como #!/bin/sh", subject)
	}
	return contents, nil
}

func executableDestinationAvailable(path, subject string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("o %s já existe: %s", subject, path)
}

func validatePersonalCommandParents(stateDir string) error {
	for _, relative := range []string{"scripts", filepath.Join("scripts", "bin")} {
		path := filepath.Join(stateDir, relative)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("a pasta de comandos pessoais deve ser um diretório real: %s", path)
		}
	}
	return nil
}

func createPersonalCommand(stateDir, path string, contents []byte) error {
	if err := ensurePersonalCommandParents(stateDir); err != nil {
		return err
	}
	return createExclusiveExecutable(path, contents, "comando pessoal")
}

func createExclusiveExecutable(path string, contents []byte, subject string) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".executable-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("não foi possível criar o %s: %w", subject, err)
	}
	return nil
}

func ensurePersonalCommandParents(stateDir string) error {
	for _, relative := range []string{"scripts", filepath.Join("scripts", "bin")} {
		path := filepath.Join(stateDir, relative)
		if err := os.Mkdir(path, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("a pasta de comandos pessoais deve ser um diretório real: %s", path)
		}
	}
	return nil
}

func ensurePersonalCommandsOnPath(contents []byte) ([]byte, bool, error) {
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return nil, false, fmt.Errorf("mise.toml inválido: %w", err)
	}
	value, exists, err := personalCommandsPathValue(document)
	if err != nil {
		return nil, false, err
	}
	if exists {
		if personalCommandsPathIncludesState(value) {
			return append([]byte(nil), contents...), false, nil
		}
		return nil, false, errors.New("env._.path já existe sem scripts/bin; acrescente a pasta manualmente, execute `konen trust` e tente novamente")
	}
	literal, err := state.TOMLString(personalCommandPathExpression)
	if err != nil {
		return nil, false, err
	}
	after, _, err := state.AddTableEntry(contents, []string{"env", "_"}, "path", literal)
	if err != nil {
		return nil, false, fmt.Errorf("não foi possível expor scripts/bin no PATH: %w", err)
	}
	return after, true, nil
}

func personalCommandsPathValue(document map[string]any) (any, bool, error) {
	envValue, exists := document["env"]
	if !exists {
		return nil, false, nil
	}
	env, ok := envValue.(map[string]any)
	if !ok {
		return nil, false, errors.New("a declaração env do mise.toml não é uma tabela")
	}
	underscoreValue, exists := env["_"]
	if !exists {
		return nil, false, nil
	}
	underscore, ok := underscoreValue.(map[string]any)
	if !ok {
		return nil, false, errors.New("a declaração env._ do mise.toml não é uma tabela")
	}
	value, exists := underscore["path"]
	return value, exists, nil
}

func personalCommandsPathIncludesState(value any) bool {
	switch typed := value.(type) {
	case string:
		portable := strings.TrimRight(filepath.ToSlash(strings.TrimSpace(typed)), "/")
		return portable == "scripts/bin" || strings.HasSuffix(portable, "/scripts/bin")
	case []any:
		for _, item := range typed {
			if personalCommandsPathIncludesState(item) {
				return true
			}
		}
	}
	return false
}
