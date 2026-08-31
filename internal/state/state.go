package state

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/roqem/konen/internal/execx"
)

const defaultMiseConfig = `min_version = "2026.8.15"

[settings]
dotfiles.root = "home"
dotfiles.default_mode = "symlink"

[dotfiles]
"~/.config/mise/config.toml" = { source = "mise.toml", mode = "symlink" }

# Declare tools, packages, repositories and managed files here.
# ` + "`konen dotfile add ~/.zshrc`" + ` adds a dotfile without inventing a second format.
`

const defaultGitignore = `.env
.env.*
*.key
*.pem
secrets/
`

type Service struct {
	Runner execx.Runner
}

func ResolvePath(input, homeDir string) (string, error) {
	if input == "" {
		return "", errors.New("o caminho do estado não pode ser vazio")
	}
	if input == "~" {
		input = homeDir
	} else if strings.HasPrefix(input, "~/") {
		input = filepath.Join(homeDir, strings.TrimPrefix(input, "~/"))
	}
	return filepath.Abs(input)
}

func (s Service) PrepareLocal(ctx context.Context, path string, initializeGit bool) error {
	if err := ensureUsableDirectory(path); err != nil {
		return err
	}

	if err := writeIfMissing(filepath.Join(path, "mise.toml"), []byte(defaultMiseConfig), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(path, "home"), 0o755); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(path, "home", ".gitkeep"), nil, 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(path, "projects"), 0o755); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(path, "projects", ".gitkeep"), nil, 0o644); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(path, ".gitignore"), []byte(defaultGitignore), 0o644); err != nil {
		return err
	}

	if initializeGit {
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if _, err := s.Runner.LookPath("git"); err != nil {
			return errors.New("git não está instalado")
		}
		if err := s.Runner.Run(ctx, path, "git", "init", "--initial-branch=main"); err != nil {
			return fmt.Errorf("git init: %w", err)
		}
	}
	return nil
}

func (s Service) Clone(ctx context.Context, remote, path string) error {
	return s.clone(ctx, remote, path, nil)
}

func (s Service) CloneWithoutPrompt(ctx context.Context, remote, path string) error {
	return s.clone(ctx, remote, path, []string{"GIT_TERMINAL_PROMPT=0"})
}

func (s Service) clone(ctx context.Context, remote, path string, environment []string) error {
	if remote == "" {
		return errors.New("a origem Git não pode ser vazia")
	}
	if _, err := s.Runner.LookPath("git"); err != nil {
		return errors.New("git não está instalado")
	}
	if _, err := os.Stat(path); err == nil {
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		if len(entries) != 0 {
			return fmt.Errorf("o destino não está vazio: %s", path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	var cloneErr error
	if len(environment) == 0 {
		cloneErr = s.Runner.Run(ctx, "", "git", "clone", "--", remote, path)
	} else {
		cloneErr = s.Runner.RunEnv(ctx, "", environment, "git", "clone", "--", remote, path)
	}
	if cloneErr != nil {
		return fmt.Errorf("git clone: %w", cloneErr)
	}
	if _, err := os.Stat(filepath.Join(path, "mise.toml")); err != nil {
		return fmt.Errorf("o repositório foi clonado, mas não contém mise.toml: %w", err)
	}
	return nil
}

func Valid(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("o estado não é um diretório: %s", path)
	}
	if _, err := os.Stat(filepath.Join(path, "mise.toml")); err != nil {
		return fmt.Errorf("mise.toml não encontrado em %s", path)
	}
	return nil
}

func ensureUsableDirectory(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.MkdirAll(path, 0o755)
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("o caminho não é um diretório: %s", path)
	}

	if _, err := os.Stat(filepath.Join(path, "mise.toml")); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != ".git" {
			return fmt.Errorf("%s não está vazio e ainda não é um estado Konen (mise.toml ausente)", path)
		}
	}
	return nil
}

func writeIfMissing(path string, data []byte, mode fs.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, mode)
}
