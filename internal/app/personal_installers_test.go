package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/state"
)

func TestPersonalInstallerAddDryRunShowsExecutableAndBootstrapWithoutWriting(t *testing.T) {
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
	configPath := filepath.Join(stateDir, "mise.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{
		"installer", "add", "--dry-run", "browser",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("dry run changed mise.toml:\n%s", after)
	}
	destination := filepath.Join(stateDir, "mise-tasks", "install", "browser")
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("dry run created the installer: %v", err)
	}
	for _, fragment := range []string{
		"+++ mise-tasks/install/browser (proposto)",
		`+#MISE description="Instalador pessoal browser (não implementado)"`,
		"+[tasks.bootstrap]", `+  { task = "install:browser" },`,
		"termina com erro até ser implementado",
		"Nenhum arquivo foi gravado e nenhuma tarefa foi executada",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("installer dry-run output is missing %q:\n%s", fragment, out.String())
		}
	}
	if len(runner.runs) != 0 {
		t.Fatalf("installer dry run invoked external commands: %#v", runner.runs)
	}
}

func TestPersonalInstallerAddCreatesSelectedExecutableWithoutRunningIt(t *testing.T) {
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
		"installer", "add", "--yes", "browser",
	}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(stateDir, "mise-tasks", "install", "browser")
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("installer mode = %04o, want 0755", got)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, personalInstallerScaffold("browser")) {
		t.Fatalf("unexpected installer scaffold:\n%s", contents)
	}
	config, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), `{ task = "install:browser" }`) {
		t.Fatalf("installer was not selected in bootstrap:\n%s", config)
	}
	if len(runner.runs) != 1 || runner.runs[0].name != "/bin/mise" || runner.runs[0].args[0] != "trust" {
		t.Fatalf("installer add did more than refresh mise trust: %#v", runner.runs)
	}
	trusted, err := application.stateTrust().IsTrusted(stateDir)
	if err != nil || !trusted {
		t.Fatalf("guided installer was not trusted: trusted=%v err=%v", trusted, err)
	}
	status, err := formatMiseStatusWithState([]byte(`{}`), stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "Instalador pessoal") || !strings.Contains(status, "install:browser") {
		t.Fatalf("status does not expose the installer:\n%s", status)
	}
	if !strings.Contains(out.String(), "não foi executado durante o cadastro") {
		t.Fatalf("result does not explain the execution boundary: %s", out.String())
	}
}

func TestPersonalInstallerAddImportsExactTextAndAppendsSequentialTask(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	source := filepath.Join(root, "install-example")
	contents := []byte("#!/bin/sh\n#MISE description=\"Instala Example\"\nset -eu\nexit 0\n")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(stateDir, "mise.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config, _, err = state.AddTaskRunReference(config, "install:existing")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := application.stateTrust().Trust(stateDir); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	if err := application.Run(context.Background(), []string{
		"installer", "add", "--yes", "--from", source, "example",
	}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(stateDir, "mise-tasks", "install", "example")
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("import changed the installer bytes:\ngot:  %q\nwant: %q", got, contents)
	}
	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	existing := strings.Index(string(updated), `task = "install:existing"`)
	added := strings.Index(string(updated), `task = "install:example"`)
	if existing < 0 || added <= existing {
		t.Fatalf("installer tasks are not sequential in insertion order:\n%s", updated)
	}
}

func TestPersonalInstallerAddRequiresExplicitNonInteractivePermission(t *testing.T) {
	application := New(Options{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Prompter: unusedPrompter{}})
	err := application.Run(context.Background(), []string{"installer", "add", "browser"})
	if err == nil || !strings.Contains(err.Error(), "--dry-run") || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("non-interactive installer error = %v", err)
	}
}
