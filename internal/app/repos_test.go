package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoAddWritesPortableDeclarationWithoutCloning(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	destination := filepath.Join(root, "Projects", "sample")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "git": "/bin/git"}}
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
		"repo", "add", "--yes", destination, "https://example.test/sample.git", "main",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"~/Projects/sample" = { url = "https://example.test/sample.git", ref = "main" }`
	if !strings.Contains(string(data), want) {
		t.Fatalf("repo declaration is missing:\n%s", data)
	}
	if len(runner.runs) != 1 || runner.runs[0].name != "/bin/mise" || runner.runs[0].args[0] != "trust" {
		t.Fatalf("repo add cloned or ran an unexpected command: %#v", runner.runs)
	}
	if !strings.Contains(out.String(), "Nenhum clone foi feito") {
		t.Fatalf("repo result does not explain deferred clone: %s", out.String())
	}
}

func TestRepoAddDryRunDoesNotWrite(t *testing.T) {
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
	before, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	if err := application.Run(context.Background(), []string{
		"repo", "add", "--dry-run", "~/src/sample", "git@example.test:sample.git",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("repo dry run changed mise.toml:\n%s", after)
	}
	if !strings.Contains(out.String(), "+[bootstrap.repos]") || len(runner.runs) != 0 {
		t.Fatalf("unexpected repo dry-run result: output=%s runs=%#v", out.String(), runner.runs)
	}
}

func TestRepositoryDestinationMustBePortableOrAbsolute(t *testing.T) {
	if _, err := normalizeRepositoryDestination("relative/path", "/home/test"); err == nil {
		t.Fatal("relative repo destination was accepted")
	}
	got, err := normalizeRepositoryDestination("/home/test/src/app", "/home/test")
	if err != nil || got != "~/src/app" {
		t.Fatalf("normalized destination = %q, %v", got, err)
	}
}
