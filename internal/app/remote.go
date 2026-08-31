package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const githubCLITool = "gh@latest"

type remoteSource struct {
	cloneURL         string
	githubRepository string
}

type githubCommand struct {
	name   string
	prefix []string
}

func (a *App) cloneRemoteState(ctx context.Context, input, destination string) error {
	source, err := parseRemoteSource(input)
	if err != nil {
		return err
	}
	clone := a.state.Clone
	if source.githubRepository != "" {
		clone = a.state.CloneWithoutPrompt
	}
	cloneErr := clone(ctx, source.cloneURL, destination)
	if cloneErr == nil || source.githubRepository == "" {
		return cloneErr
	}
	if !a.options.Interactive {
		return fmt.Errorf("%w; a origem GitHub pode exigir autenticação: execute novamente em um terminal interativo ou autentique o Git antes", cloneErr)
	}

	fmt.Fprintln(a.options.Out, "O clone HTTPS falhou. O Konen tentará autenticar no GitHub sem criar uma chave SSH.")
	if err := a.authenticateGitHub(ctx, source.githubRepository); err != nil {
		return fmt.Errorf("clone inicial: %v; autenticação no GitHub: %w", cloneErr, err)
	}
	if err := clone(ctx, source.cloneURL, destination); err != nil {
		return fmt.Errorf("clone após autenticação no GitHub: %w", err)
	}
	return nil
}

func (a *App) authenticateGitHub(ctx context.Context, repository string) error {
	command, err := a.githubCLI()
	if err != nil {
		return err
	}
	run := func(args ...string) error {
		fullArgs := append(append([]string(nil), command.prefix...), args...)
		return a.options.Runner.Run(ctx, "", command.name, fullArgs...)
	}
	output := func(args ...string) error {
		fullArgs := append(append([]string(nil), command.prefix...), args...)
		_, err := a.options.Runner.Output(ctx, "", command.name, fullArgs...)
		return err
	}
	canAccessRepository := func() bool {
		return output("repo", "view", repository, "--json", "nameWithOwner") == nil
	}

	if !canAccessRepository() {
		if err := run("auth", "status", "--hostname", "github.com"); err == nil {
			fmt.Fprintf(a.options.Out, "A conta ativa do GitHub não acessa %s; selecione outra conta autenticada.\n", repository)
			if err := run("auth", "switch", "--hostname", "github.com"); err != nil {
				fmt.Fprintln(a.options.Out, "Nenhuma outra conta autenticada resolveu o acesso; será iniciado um novo login.")
				if err := a.loginGitHub(run); err != nil {
					return err
				}
			}
		} else if err := a.loginGitHub(run); err != nil {
			return err
		}
		if !canAccessRepository() {
			return fmt.Errorf("a conta ativa do GitHub ainda não pode acessar %s; confirme o proprietário, o nome e as permissões do repositório", repository)
		}
	}

	fmt.Fprintln(a.options.Out, "Configurando o Git para usar as credenciais HTTPS da conta ativa no GitHub CLI.")
	if err := run("auth", "setup-git", "--hostname", "github.com"); err != nil {
		return fmt.Errorf("gh auth setup-git: %w", err)
	}
	return nil
}

func (a *App) loginGitHub(run func(...string) error) error {
	fmt.Fprintln(a.options.Out, "Autorize o GitHub com o código exibido; o navegador pode estar em outro dispositivo.")
	if err := run("auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web"); err != nil {
		return fmt.Errorf("gh auth login: %w", err)
	}
	return nil
}

func (a *App) githubCLI() (githubCommand, error) {
	if path, err := a.findCommand("gh"); err == nil {
		return githubCommand{name: path}, nil
	}
	misePath, err := a.findCommand("mise")
	if err != nil {
		return githubCommand{}, errors.New("GitHub CLI e mise não foram encontrados; instale `gh` e execute `gh auth login --git-protocol https`")
	}
	fmt.Fprintf(a.options.Out, "GitHub CLI não encontrado; o mise baixará %s sem sudo para esta autenticação.\n", githubCLITool)
	return githubCommand{
		name:   misePath,
		prefix: []string{"exec", "--yes", "--raw", githubCLITool, "--", "gh"},
	}, nil
}

func parseRemoteSource(input string) (remoteSource, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return remoteSource{}, errors.New("a origem Git não pode ser vazia")
	}
	if spec, found := strings.CutPrefix(input, "github:"); found {
		owner, repository, err := parseGitHubRepository(spec)
		if err != nil {
			return remoteSource{}, err
		}
		return remoteSource{
			cloneURL:         fmt.Sprintf("https://github.com/%s/%s.git", owner, repository),
			githubRepository: owner + "/" + repository,
		}, nil
	}

	parsed, err := url.Parse(input)
	if err == nil && parsed.Scheme == "https" && strings.EqualFold(parsed.Host, "github.com") {
		owner, repository, repositoryErr := parseGitHubRepository(strings.TrimPrefix(parsed.Path, "/"))
		if repositoryErr == nil && parsed.RawQuery == "" && parsed.Fragment == "" {
			return remoteSource{
				cloneURL:         fmt.Sprintf("https://github.com/%s/%s.git", owner, repository),
				githubRepository: owner + "/" + repository,
			}, nil
		}
	}
	return remoteSource{cloneURL: input}, nil
}

func parseGitHubRepository(spec string) (string, string, error) {
	spec = strings.TrimSuffix(strings.TrimSpace(spec), ".git")
	parts := strings.Split(spec, "/")
	if len(parts) != 2 || !validGitHubName(parts[0]) || !validGitHubName(parts[1]) {
		return "", "", fmt.Errorf("origem GitHub inválida %q; use github:OWNER/REPO", spec)
	}
	return parts[0], parts[1], nil
}

func validGitHubName(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '-', character == '_', character == '.':
		default:
			return false
		}
	}
	return true
}
