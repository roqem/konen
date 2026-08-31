package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roqem/konen/internal/ui"
)

func (a *App) reviewStateConfigChange(
	ctx context.Context,
	stateDir, misePath string,
	before, after []byte,
	dryRun, yes bool,
	confirmation string,
) (bool, error) {
	configPath := filepath.Join(stateDir, "mise.toml")
	fmt.Fprintf(a.options.Out, "Alteração proposta em %s:\n", configPath)
	fmt.Fprint(a.options.Out, ui.RenderDiff("mise.toml", string(before), string(after)))
	if dryRun {
		fmt.Fprintln(a.options.Out, "Nenhuma alteração foi gravada.")
		return false, nil
	}
	if !yes {
		confirmed, err := a.options.Prompter.Confirm(confirmation)
		if err != nil {
			return false, err
		}
		if !confirmed {
			fmt.Fprintln(a.options.Out, "Alteração cancelada.")
			return false, nil
		}
	}

	current, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(before, current) {
		return false, errors.New("mise.toml mudou durante a revisão; execute o comando novamente")
	}
	if err := replaceFile(configPath, after); err != nil {
		return false, err
	}
	if err := a.options.Runner.RunEnv(
		ctx, stateDir, miseStateEnvironment(stateDir), misePath, "trust", configPath,
	); err != nil {
		return false, fmt.Errorf("estado gravado, mas não foi possível confiar no mise.toml: %w", err)
	}
	if _, err := a.stateTrust().Trust(stateDir); err != nil {
		return false, fmt.Errorf("estado gravado, mas a aprovação local não pôde ser atualizada: %w", err)
	}
	return true, nil
}

func replaceFile(path string, contents []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".mise-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
