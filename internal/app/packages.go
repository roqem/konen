package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

func (a *App) runPackage(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("informe uma ação: add")
	}
	if args[0] != "add" {
		return fmt.Errorf("ação de pacote desconhecida: %s", args[0])
	}
	return a.runPackageAdd(ctx, args[1:])
}

func (a *App) runPackageAdd(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("package add", flag.ContinueOnError)
	flags.SetOutput(a.options.Err)
	yes := flags.Bool("yes", false, "grava sem pedir confirmação")
	dryRun := flags.Bool("dry-run", false, "mostra a alteração sem gravar")
	manager := flags.String("manager", "", "gerenciador do sistema, como apt ou brew")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 2 {
		return errors.New("uso: konen package add [--manager GERENCIADOR] [--dry-run] PACOTE [VERSÃO]")
	}
	if !a.options.Interactive && !*dryRun && !*yes {
		return errors.New("em modo não interativo, revise com `--dry-run` ou confirme a gravação com `--yes`")
	}

	stateDir, misePath, err := a.loadTrustedMise(ctx)
	if err != nil {
		return err
	}
	answer := ui.PackageAnswer{Manager: strings.TrimSpace(*manager), Version: "latest"}
	if answer.Manager == "" {
		answer.Manager = a.defaultPackageManager()
	}
	if flags.NArg() >= 1 {
		answer.Name = flags.Arg(0)
	}
	if flags.NArg() == 2 {
		answer.Version = flags.Arg(1)
	}
	if answer.Name == "" {
		if !a.options.Interactive {
			return errors.New("informe o pacote: konen package add [--manager GERENCIADOR] PACOTE [VERSÃO]")
		}
		answer, err = a.options.Prompter.Package(answer)
		if err != nil {
			return err
		}
	}
	answer.Manager = strings.TrimSpace(answer.Manager)
	answer.Name = strings.TrimSpace(answer.Name)
	answer.Version = strings.TrimSpace(answer.Version)
	if answer.Version == "" {
		answer.Version = "latest"
	}
	if err := validatePackageAnswer(answer); err != nil {
		return err
	}

	configPath := filepath.Join(stateDir, "mise.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	literal, err := state.TOMLString(answer.Version)
	if err != nil {
		return err
	}
	key := answer.Manager + ":" + answer.Name
	after, exists, err := state.AddTableEntry(before, []string{"bootstrap", "packages"}, key, literal)
	if err != nil {
		return fmt.Errorf("mise.toml inválido: %w", err)
	}
	if exists {
		fmt.Fprintf(a.options.Out, "Pacote já declarado: %s\n", key)
		return nil
	}

	fmt.Fprintf(a.options.Out, "Pacote: %s\n", answer.Name)
	fmt.Fprintf(a.options.Out, "Gerenciador: %s — %s\n", answer.Manager, packageManagerDescription(answer.Manager))
	fmt.Fprintf(a.options.Out, "Versão: %s\n", answer.Version)
	written, err := a.reviewStateConfigChange(
		ctx, stateDir, misePath, before, after, *dryRun, *yes,
		"Gravar este pacote no estado?",
	)
	if err != nil || !written {
		return err
	}
	fmt.Fprintf(a.options.Out, "Pacote adicionado ao estado: %s@%s\n", key, answer.Version)
	fmt.Fprintln(a.options.Out, "Nenhum pacote foi instalado. Próximo passo: execute `konen plan --only packages`.")
	return nil
}

func (a *App) defaultPackageManager() string {
	candidates := []struct {
		manager string
		command string
	}{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, struct{ manager, command string }{"brew", "brew"})
	case "linux":
		candidates = append(candidates,
			struct{ manager, command string }{"apt", "apt-get"},
			struct{ manager, command string }{"dnf", "dnf"},
			struct{ manager, command string }{"pacman", "pacman"},
			struct{ manager, command string }{"apk", "apk"},
			struct{ manager, command string }{"brew", "brew"},
		)
	}
	for _, candidate := range candidates {
		if _, err := a.options.Runner.LookPath(candidate.command); err == nil {
			return candidate.manager
		}
	}
	if runtime.GOOS == "darwin" {
		return "brew"
	}
	return "apt"
}

func validatePackageAnswer(answer ui.PackageAnswer) error {
	if answer.Manager == "" || answer.Name == "" {
		return errors.New("gerenciador e pacote são obrigatórios")
	}
	if !validManagerName(answer.Manager) {
		return errors.New("o gerenciador aceita apenas letras, números, hífen e sublinhado")
	}
	if answer.Version == "" {
		return errors.New("a versão não pode ser vazia")
	}
	for _, value := range []string{answer.Name, answer.Version} {
		if strings.HasPrefix(value, "-") {
			return errors.New("pacote e versão não podem começar com hífen")
		}
		for _, character := range value {
			if unicode.IsSpace(character) || unicode.IsControl(character) {
				return errors.New("pacote e versão não podem conter espaços ou caracteres de controle")
			}
		}
	}
	if answer.Manager == "mas" && answer.Version != "latest" {
		return errors.New("pacotes da Mac App Store não aceitam versão; use latest")
	}
	return nil
}

func validManagerName(value string) bool {
	for index, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' {
			if index > 0 || unicode.IsLetter(character) || unicode.IsDigit(character) {
				continue
			}
		}
		return false
	}
	return value != ""
}

func packageManagerDescription(manager string) string {
	descriptions := map[string]string{
		"apk":          "Alpine Linux; a instalação pode usar sudo",
		"apt":          "Debian e Ubuntu; a instalação pode usar sudo",
		"dnf":          "Fedora e derivados; a instalação pode usar sudo",
		"pacman":       "Arch e derivados; a instalação pode usar sudo",
		"brew":         "Homebrew no macOS ou Linux",
		"brew-cask":    "aplicativos e fontes distribuídos como casks",
		"flatpak":      "Flatpak do sistema no Linux",
		"flatpak-user": "Flatpak apenas deste usuário",
		"mas":          "Mac App Store pelo identificador numérico",
	}
	if description, ok := descriptions[manager]; ok {
		return description
	}
	return "gerenciador fornecido por um plugin do mise"
}
