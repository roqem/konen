package app

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/ui"
)

type applyPrompter struct {
	unusedPrompter
	offered  []ui.ApplyPart
	selected []string
}

func (p *applyPrompter) ChooseApplyParts(parts []ui.ApplyPart) ([]string, error) {
	p.offered = append([]ui.ApplyPart(nil), parts...)
	return append([]string(nil), p.selected...), nil
}

func TestPlanSelectUsesOnlyTheChosenDeclaredParts(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	prompter := &applyPrompter{selected: []string{"dotfiles"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath:  filepath.Join(root, "config.toml"),
		HomeDir:     root,
		Out:         &out,
		Err:         &out,
		Runner:      runner,
		Prompter:    prompter,
		Interactive: true,
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{"plan", "--select"}); err != nil {
		t.Fatal(err)
	}
	if len(prompter.offered) != 1 || prompter.offered[0].Key != "dotfiles" {
		t.Fatalf("offered parts = %#v", prompter.offered)
	}
	want := runCall{
		dir:         stateDir,
		environment: miseStateEnvironment(stateDir),
		name:        "/bin/mise",
		args:        []string{"-C", stateDir, "bootstrap", "--only", "dotfiles", "--dry-run"},
	}
	if len(runner.runs) != 1 || !reflect.DeepEqual(runner.runs[0], want) {
		t.Fatalf("runs = %#v, want final call %#v", runner.runs, want)
	}
	if !strings.Contains(out.String(), "Etapas selecionadas: Dotfiles") {
		t.Fatalf("selection output = %q", out.String())
	}
}

func TestPlanOnlyWorksWithoutAnInteractiveTerminal(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &bytes.Buffer{}, Err: &bytes.Buffer{},
		Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	if err := application.Run(context.Background(), []string{"plan", "--only", "tools,dotfiles"}); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"-C", stateDir, "bootstrap", "--only", "tools,dotfiles", "--dry-run"}
	if len(runner.runs) != 1 || !reflect.DeepEqual(runner.runs[0].args, wantArgs) {
		t.Fatalf("runs = %#v, want final args %#v", runner.runs, wantArgs)
	}
}

func TestEmptyApplySelectionDoesNothing(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	prompter := &applyPrompter{}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &out, Err: &out, Runner: runner,
		Prompter: prompter, Interactive: true,
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{"apply", "--select"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("empty selection invoked mise: %#v", runner.runs)
	}
	if !strings.Contains(out.String(), "Nenhuma etapa selecionada") {
		t.Fatalf("empty selection output = %q", out.String())
	}
}

func TestDeclaredApplyPartsFollowTheStateContents(t *testing.T) {
	parts, err := declaredApplyParts([]byte(`
[tools]
node = "lts"

[bootstrap.packages]
"apt:git" = "latest"

[bootstrap.repos]
"~/src/example" = { url = "https://example.test/repo.git" }

[dotfiles]
"~/.zshrc" = { mode = "copy" }

[tasks.bootstrap]
run = [{ task = "install:browser" }]
`))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, part := range parts {
		keys = append(keys, part.Key)
	}
	want := []string{"packages", "repos", "dotfiles", "tools", "task"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("declared parts = %#v, want %#v", keys, want)
	}
}

func TestMiseStateEnvironmentExcludesOtherMachineConfigs(t *testing.T) {
	stateDir := "/tmp/example-state"
	want := []string{
		"MISE_GLOBAL_CONFIG_FILE=/tmp/example-state/mise.toml",
		"MISE_GLOBAL_CONFIG_ROOT=/tmp/example-state",
		"MISE_CEILING_PATHS=/tmp/example-state",
		"MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml",
	}
	if got := miseStateEnvironment(stateDir); !reflect.DeepEqual(got, want) {
		t.Fatalf("mise environment = %#v, want %#v", got, want)
	}
}
