package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/ui"
)

func TestPackageAddDryRunShowsDiffWithoutWritingOrInstalling(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "apt-get": "/usr/bin/apt-get"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{
		"package", "add", "--dry-run", "--manager", "apt", "jq", "latest",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("dry run changed mise.toml:\n%s", after)
	}
	for _, fragment := range []string{
		"Gerenciador: apt", "+[bootstrap.packages]", `+"apt:jq" = "latest"`,
		"Nenhuma alteração foi gravada",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("dry-run output is missing %q:\n%s", fragment, out.String())
		}
	}
	if len(runner.runs) != 0 {
		t.Fatalf("package dry run invoked external commands: %#v", runner.runs)
	}
}

func TestPackageAddWritesStateButDoesNotInstall(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{
		"package", "add", "--yes", "--manager", "apt", "jq",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"apt:jq" = "latest"`) {
		t.Fatalf("package was not written:\n%s", data)
	}
	if len(runner.runs) != 1 || len(runner.runs[0].args) != 2 || runner.runs[0].args[0] != "trust" {
		t.Fatalf("package add did more than refresh trust: %#v", runner.runs)
	}
	if !strings.Contains(out.String(), "Nenhum pacote foi instalado") {
		t.Fatalf("package result is not explicit about deferred install: %s", out.String())
	}
}

func TestPackageAddRequiresExplicitNonInteractivePermission(t *testing.T) {
	application := New(Options{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Prompter: unusedPrompter{}})
	err := application.Run(context.Background(), []string{"package", "add", "jq"})
	if err == nil || !strings.Contains(err.Error(), "--dry-run") || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("non-interactive package error = %v", err)
	}
}

func TestPackageValidationRejectsInvalidManagerAndMasPin(t *testing.T) {
	for _, answer := range []struct {
		value ui.PackageAnswer
		want  string
	}{
		{ui.PackageAnswer{Manager: "bad.manager", Name: "jq", Version: "latest"}, "gerenciador"},
		{ui.PackageAnswer{Manager: "mas", Name: "497799835", Version: "1"}, "não aceitam versão"},
	} {
		err := validatePackageAnswer(answer.value)
		if err == nil || !strings.Contains(err.Error(), answer.want) {
			t.Errorf("validatePackageAnswer(%#v) = %v", answer.value, err)
		}
	}
}
