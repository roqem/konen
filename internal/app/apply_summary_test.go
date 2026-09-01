package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildApplySummaryDistinguishesConvergencePendingAndTasks(t *testing.T) {
	before := []statusRow{
		{kind: "Ferramenta", item: "node", state: "ausente", part: "tools"},
		{kind: "Dotfile", item: "~/.zshrc", state: "diferente", part: "dotfiles"},
		{kind: "Pacote (apt)", item: "git", state: "instalado", part: "packages"},
		{kind: "Shell de login", item: "user", state: "diferente", part: "user"},
	}
	after := []statusRow{
		{kind: "Ferramenta", item: "node", state: "instalado", part: "tools"},
		{kind: "Dotfile", item: "~/.zshrc", state: "aplicado", part: "dotfiles"},
		{kind: "Pacote (apt)", item: "git", state: "instalado", part: "packages"},
		{kind: "Repositório", item: "~/src/example", state: "ausente", part: "repos"},
	}

	summary := buildApplySummary(before, after, nil, true)
	if len(summary.changed) != 3 || len(summary.ready) != 1 || len(summary.pendingScope) != 1 ||
		len(summary.pendingAll) != 1 || !summary.taskRan || !summary.loginChanged {
		t.Fatalf("apply summary = %#v", summary)
	}
	got := renderApplySummary(summary)
	for _, fragment := range []string{
		"Resumo da aplicação", "Convergiram nesta execução", "3", "Já estavam prontos",
		"Ainda pendentes nas etapas aplicadas", "Etapa de tarefas pessoais",
		"Recursos pendentes", "Nova sessão de login", "Mensagens das tarefas",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("rendered apply summary is missing %q:\n%s", fragment, got)
		}
	}
}

func TestBuildApplySummaryScopesComparisonButKeepsWholeStatePendingVisible(t *testing.T) {
	before := []statusRow{
		{kind: "Ferramenta", item: "node", state: "ausente", part: "tools"},
		{kind: "Dotfile", item: "~/.zshrc", state: "diferente", part: "dotfiles"},
	}
	after := []statusRow{
		{kind: "Ferramenta", item: "node", state: "instalado", part: "tools"},
		{kind: "Dotfile", item: "~/.zshrc", state: "diferente", part: "dotfiles"},
	}
	summary := buildApplySummary(before, after, []string{"tools"}, false)
	if len(summary.changed) != 1 || len(summary.pendingScope) != 0 || len(summary.pendingAll) != 1 {
		t.Fatalf("scoped apply summary = %#v", summary)
	}
}

func TestApplyDoesNotInspectMiseAgainWhenExecutableStateChanges(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	var out bytes.Buffer
	runner := &fakeRunner{
		paths:   map[string]string{"mise": "/bin/mise"},
		outputs: map[string]string{"/bin/mise": `{"tools":[]}`},
		runHook: func(call runCall) error {
			if strings.Contains(" "+strings.Join(call.args, " ")+" ", " bootstrap --yes ") {
				return os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\nnode = \"lts\"\n"), 0o644)
			}
			return nil
		},
	}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{"apply", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 2 {
		t.Fatalf("changed executable state was inspected after apply: %#v", runner.runs)
	}
	for _, fragment := range []string{"uma tarefa ou um comando pessoal mudou", "konen trust", "não consultou o mise novamente"} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("changed-state guidance is missing %q: %s", fragment, out.String())
		}
	}
}

func TestApplyCompletesWhenStructuredSummaryIsUnavailable(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	var out bytes.Buffer
	runner := &fakeRunner{
		paths:      map[string]string{"mise": "/bin/mise"},
		outputHook: func(runCall) (string, error) { return "", errors.New("status unavailable") },
	}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{"apply", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 3 {
		t.Fatalf("apply did not attempt before/apply/after in order: %#v", runner.runs)
	}
	if !strings.Contains(out.String(), "A aplicação terminou") || !strings.Contains(out.String(), "konen status") {
		t.Fatalf("unavailable summary output = %s", out.String())
	}
}

func TestFailedApplyDoesNotClaimSuccessOrQueryPostStatus(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	var out bytes.Buffer
	runner := &fakeRunner{
		paths:   map[string]string{"mise": "/bin/mise"},
		outputs: map[string]string{"/bin/mise": `{"tools":[]}`},
		runHook: func(call runCall) error {
			if strings.Contains(" "+strings.Join(call.args, " ")+" ", " bootstrap --yes ") {
				return errors.New("apply failed")
			}
			return nil
		},
	}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	err := application.Run(context.Background(), []string{"apply", "--yes"})
	if err == nil || !strings.Contains(err.Error(), "apply failed") {
		t.Fatalf("failed apply error = %v", err)
	}
	if len(runner.runs) != 2 {
		t.Fatalf("failed apply queried post status: %#v", runner.runs)
	}
	if strings.Contains(out.String(), "Resumo da aplicação") || strings.Contains(out.String(), "A aplicação terminou") {
		t.Fatalf("failed apply claimed success: %s", out.String())
	}
}
