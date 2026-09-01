package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roqem/konen/internal/app"
	"github.com/roqem/konen/internal/config"
	"github.com/roqem/konen/internal/execx"
	"github.com/roqem/konen/internal/ui"
)

var version = "dev"

func main() {
	configPath, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: não foi possível localizar a configuração: %v\n", err)
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: não foi possível localizar seu diretório pessoal: %v\n", err)
		os.Exit(1)
	}

	runner := execx.OSRunner{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
	executable := executablePath()
	application := app.New(app.Options{
		ConfigPath:     configPath,
		HomeDir:        homeDir,
		In:             os.Stdin,
		Out:            os.Stdout,
		Err:            os.Stderr,
		Runner:         runner,
		Prompter:       ui.NewHuhPrompter(os.Stdin, os.Stderr),
		Interactive:    isTerminal(os.Stdin),
		Version:        version,
		BinDir:         executableDir(executable),
		ExecutablePath: executable,
	})

	if err := application.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func executablePath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return executable
}

func executableDir(executable string) string {
	if executable == "" {
		return ""
	}
	return filepath.Dir(executable)
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
