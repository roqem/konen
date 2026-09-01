package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/project"
)

func TestCompletionScriptsCoverCommandsAndDynamicProjects(t *testing.T) {
	common := []string{
		"init", "status", "plan", "diff", "apply", "migrate", "update", "tool", "package", "repo", "command", "installer", "dotfile", "add", "projects", "project", "dev", "run",
		"trust", "doctor", "completion", "version", "help",
		"edit", "list", "show", "__complete projects", "__complete actions",
	}
	options := map[string][]string{
		"zsh":  {"--git", "--from", "--yes", "--dry-run", "--select", "--only", "--state", "ready", "pending", "unknown", "--pre", "konen mise", "--manager", "--mode"},
		"bash": {"--git", "--from", "--yes", "--dry-run", "--select", "--only", "--state", "ready", "pending", "unknown", "--pre", "konen mise", "--manager", "--mode"},
		"fish": {"-l git", "-l from", "-l yes", "-l dry-run", "-l select", "-l only", "-l state", "ready", "pending", "unknown", "-l pre", "konen mise", "-l manager", "-l mode"},
	}
	for _, shell := range []string{"zsh", "bash", "fish"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range append(common, options[shell]...) {
			if !strings.Contains(script, fragment) {
				t.Errorf("%s completion is missing %q", shell, fragment)
			}
		}
	}
}

func TestInternalCompletionListsActionsForProjectAndCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "sample")
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root, WorkDir: projectDir,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: root}
	if _, err := store.Save("sample", project.Manifest{
		Version: 2, Path: projectDir,
		Actions: map[string]project.Action{"test": {Task: "test"}, "console": {Task: "dev:console"}},
		Tabs:    []project.Tab{{Title: "Terminal"}},
	}); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"__complete", "actions"}, {"__complete", "actions", "sample"}} {
		out.Reset()
		if err := application.Run(context.Background(), args); err != nil {
			t.Fatal(err)
		}
		if got, want := out.String(), "console\ntest\n"; got != want {
			t.Fatalf("action completion = %q, want %q", got, want)
		}
	}
}

func TestCompletionScriptRejectsUnsupportedShell(t *testing.T) {
	if _, err := completionScript("powershell"); err == nil {
		t.Fatal("completionScript() should reject unsupported shells")
	}
}

func TestInternalCompletionListsProjectNamesOnly(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: root}
	for _, name := range []string{"zeta", "alpha"} {
		if _, err := store.Save(name, project.Manifest{
			Version: 2, Path: root, Tabs: []project.Tab{{Title: "Terminal"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{"__complete", "projects"}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "alpha\nzeta\n"; got != want {
		t.Fatalf("project completion = %q, want %q", got, want)
	}
}
