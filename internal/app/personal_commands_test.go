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

func TestPersonalCommandAddDryRunShowsCompleteFileWithoutWriting(t *testing.T) {
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
		"command", "add", "--dry-run", "work-note",
	}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(stateDir, "scripts", "bin", "work-note")
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("dry run created the command: %v", err)
	}
	for _, fragment := range []string{
		"Arquivo proposto", "+++ scripts/bin/work-note (proposto)",
		"+#!/bin/sh", "+exit 1", "Nenhum arquivo foi gravado",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("dry-run output is missing %q:\n%s", fragment, out.String())
		}
	}
	if len(runner.runs) != 0 {
		t.Fatalf("command dry run invoked external commands: %#v", runner.runs)
	}
}

func TestPersonalCommandAddCreatesExecutableAndRefreshesLocalTrust(t *testing.T) {
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
		"command", "add", "--yes", "work-note",
	}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(stateDir, "scripts", "bin", "work-note")
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("command mode = %04o, want 0755", got)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, personalCommandScaffold("work-note")) {
		t.Fatalf("unexpected scaffold:\n%s", contents)
	}
	trusted, err := application.stateTrust().IsTrusted(stateDir)
	if err != nil || !trusted {
		t.Fatalf("guided command was not trusted: trusted=%v err=%v", trusted, err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("creating a command executed an external command: %#v", runner.runs)
	}
	status, err := formatMiseStatusWithState([]byte(`{}`), stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "Comando pessoal") || !strings.Contains(status, "work-note") {
		t.Fatalf("status does not expose the personal command:\n%s", status)
	}
	if !strings.Contains(out.String(), "O comando não foi executado") {
		t.Fatalf("result does not explain the execution boundary: %s", out.String())
	}
}

func TestPersonalCommandAddImportsExactTextWithoutExecutingIt(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	source := filepath.Join(root, "existing-command")
	contents := []byte("#!/bin/sh\nset -eu\nprintf '%s\\n' imported\n")
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
	runner.runs = nil

	if err := application.Run(context.Background(), []string{
		"command", "add", "--yes", "--from", source,
	}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(stateDir, "scripts", "bin", "existing-command")
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("import changed the source bytes:\ngot:  %q\nwant: %q", got, contents)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("import executed an external command: %#v", runner.runs)
	}
}

func TestPersonalCommandAddConfiguresPathForAnAdoptedState(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte("min_version = \"2026.8.15\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := application.stateTrust().Trust(stateDir); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{
		"command", "add", "--yes", "hello",
	}); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "[env._]") ||
		!strings.Contains(string(config), `"path" = "{{ config_source | canonicalize | dirname }}/scripts/bin"`) {
		t.Fatalf("scripts/bin was not added to the mise environment:\n%s", config)
	}
	if len(runner.runs) != 1 || runner.runs[0].name != "/bin/mise" || runner.runs[0].args[0] != "trust" {
		t.Fatalf("path setup did more than refresh mise trust: %#v", runner.runs)
	}
	if !strings.Contains(out.String(), "também precisa expor scripts/bin") {
		t.Fatalf("path mutation was not explained: %s", out.String())
	}
}

func TestPersonalCommandAddRejectsUnsafeNameAndSource(t *testing.T) {
	if err := validatePersonalCommandAnswer(personalCommandAnswer("../escape")); err == nil {
		t.Fatal("path-like command name was accepted")
	}
	root := t.TempDir()
	source := filepath.Join(root, "fragment")
	if err := os.WriteFile(source, []byte("printf 'missing shebang\\n'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPersonalCommand(source); err == nil || !strings.Contains(err.Error(), "shebang") {
		t.Fatalf("source without shebang error = %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readPersonalCommand(link); err == nil || !strings.Contains(err.Error(), "simbólico") {
		t.Fatalf("symlink source error = %v", err)
	}
	unsafe := filepath.Join(root, "unsafe")
	if err := os.WriteFile(unsafe, []byte("#!/bin/sh\nprintf '\x1b[31m'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPersonalCommand(unsafe); err == nil || !strings.Contains(err.Error(), "controle") {
		t.Fatalf("terminal control source error = %v", err)
	}
}

func TestPersonalCommandAddRequiresExplicitNonInteractivePermission(t *testing.T) {
	application := New(Options{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Prompter: unusedPrompter{}})
	err := application.Run(context.Background(), []string{"command", "add", "hello"})
	if err == nil || !strings.Contains(err.Error(), "--dry-run") || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("non-interactive command error = %v", err)
	}
}

func personalCommandAnswer(name string) ui.PersonalCommandAnswer {
	return ui.PersonalCommandAnswer{Mode: "create", Name: name}
}
