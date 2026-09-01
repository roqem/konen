package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStatusRequestsJSONFromMise(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{
		paths: map[string]string{"mise": "/bin/mise"},
		outputs: map[string]string{
			"/bin/mise": `{"tools":[{"tool":"go","state":"installed"}]}`,
		},
	}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{"status"}); err != nil {
		t.Fatal(err)
	}
	want := runCall{
		dir: stateDir, environment: miseStateEnvironment(stateDir), name: "/bin/mise",
		args: []string{"-C", stateDir, "bootstrap", "status", "--json"},
	}
	if len(runner.runs) != 2 || !reflect.DeepEqual(runner.runs[1], want) {
		t.Fatalf("status call = %#v, want %#v", runner.runs, want)
	}
	if !strings.Contains(out.String(), "Ferramenta") || !strings.Contains(out.String(), "go") {
		t.Fatalf("status output = %q", out.String())
	}
}

func TestStatusFiltersCategoriesAndCanonicalStates(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{
		paths: map[string]string{"mise": "/bin/mise"},
		outputs: map[string]string{
			"/bin/mise": `{
  "packages": {"apt": {"packages": [
    {"package":"git", "state":"installed"}
  ]}},
  "dotfiles": {"files": [
    {"target":"~/.zshrc", "state":"differs"}
  ]},
  "tools": [
    {"tool":"node", "state":"missing"},
    {"tool":"go", "state":"installed"}
  ]
}`,
		},
	}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{
		"status", "--only", "tools,dotfiles", "--state", "missing,different",
	}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, fragment := range []string{"Ferramenta", "node", "ausente", "Dotfile", "~/.zshrc", "diferente"} {
		if !strings.Contains(got, fragment) {
			t.Errorf("filtered status is missing %q:\n%s", fragment, got)
		}
	}
	for _, unwanted := range []string{"go", "git", "Backup Git"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("filtered status contains %q:\n%s", unwanted, got)
		}
	}
}

func TestStatusStateGroupsCoverPersonalAndUnknownRows(t *testing.T) {
	rows := []statusRow{
		{item: "tool", state: "traduzido arbitrariamente", stateKey: "installed", part: "tools"},
		{item: "command", state: "disponível", stateKey: "available", part: "task"},
		{item: "source", state: "fonte ausente", stateKey: "source_missing", part: "dotfiles"},
		{item: "different", state: "diferente", stateKey: "differs", part: "dotfiles"},
		{item: "future", state: "future_state", stateKey: "future_state", part: "future"},
	}

	tests := []struct {
		state string
		want  []string
	}{
		{state: "ready", want: []string{"tool", "command"}},
		{state: "pending", want: []string{"source", "different"}},
		{state: "missing", want: []string{"source"}},
		{state: "different", want: []string{"different"}},
		{state: "unknown", want: []string{"future"}},
	}
	for _, test := range tests {
		t.Run(test.state, func(t *testing.T) {
			filtered := filterStatusRows(rows, nil, []string{test.state})
			var got []string
			for _, row := range filtered {
				got = append(got, row.item)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("filter %s = %#v, want %#v", test.state, got, test.want)
			}
		})
	}
}

func TestStatusRejectsUnknownFiltersBeforeInvokingMise(t *testing.T) {
	for _, test := range []struct {
		args     []string
		fragment string
	}{
		{args: []string{"status", "--only", "browsers"}, fragment: "categoria desconhecida"},
		{args: []string{"status", "--state", "broken"}, fragment: "situação desconhecida"},
	} {
		runner := &fakeRunner{}
		application := New(Options{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner})
		err := application.Run(context.Background(), test.args)
		if err == nil || !strings.Contains(err.Error(), test.fragment) {
			t.Fatalf("status %v error = %v", test.args, err)
		}
		if len(runner.runs) != 0 {
			t.Fatalf("invalid filter invoked commands: %#v", runner.runs)
		}
	}
}

func TestFilteredStatusExplainsAnEmptyResult(t *testing.T) {
	got := renderStatusRows(nil, true)
	if got != "Nenhum item corresponde aos filtros.\n" {
		t.Fatalf("empty filtered status = %q", got)
	}
}

func TestFormatMiseStatusUsesUnifiedTableAndKeepsUnknownKinds(t *testing.T) {
	input := []byte(`{
  "packages": {"apt": {"curl": "latest"}},
  "dotfiles": {"files": [{
    "target": "~/.zshrc", "source": "home/.zshrc",
    "mode": "copy", "state": "applied"
  }]},
  "tools": [{
    "tool": "codex", "requested_version": "latest",
    "resolved_version": "0.151.0", "state": "installed", "installed": true
  }],
  "future_kind": [{"name": "visible", "channel": "stable", "state": "ready"}]
}`)

	got, err := formatMiseStatus(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Tipo", "Item", "Configuração", "Estado",
		"Dotfile", "~/.zshrc", "Fonte: home/.zshrc", "aplicado",
		"Ferramenta", "codex", "Versão pedida: latest", "Versão resolvida: 0.151.0", "instalado",
		"Pacote · Apt", "Curl", "latest",
		"Future kind", "visible", "Channel: stable", "ready",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("formatted status is missing %q:\n%s", fragment, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("formatted status contains ANSI styling: %q", got)
	}
}

func TestStatusRefusesLocallyUnapprovedStateWithoutReadingIt(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{
		paths:      map[string]string{"mise": "/bin/mise"},
		outputHook: func(runCall) (string, error) { return "", errors.New("untrusted state was read") },
	}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\nnode = \"lts\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	err := application.Run(context.Background(), []string{"status"})
	if err == nil || !strings.Contains(err.Error(), "konen trust") {
		t.Fatalf("status error = %v", err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("untrusted status calls = %#v", runner.runs)
	}
}

func TestFormatMiseStatusHandlesEmptyState(t *testing.T) {
	got, err := formatMiseStatus([]byte(`{"tools": [], "packages": {}, "login_shell": null}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "Nenhum item declarado no estado.\n" {
		t.Fatalf("empty status = %q", got)
	}
}

func TestFormatMiseStatusIncludesPersonalCommandsAndInstallers(t *testing.T) {
	root := t.TempDir()
	files := map[string]os.FileMode{
		"mise.toml":                         0o644,
		"scripts/bin/hello":                 0o755,
		"mise-tasks/install/docker":         0o755,
		"mise-tasks/install/not-executable": 0o644,
	}
	for relative, mode := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
	}

	got, err := formatMiseStatusWithState([]byte(`{"tools": []}`), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Comando pessoal", "hello", "scripts/bin/hello", "disponível",
		"Instalador pessoal", "install:docker", "mise-tasks/install/docker",
		"install:not-executable", "não executável",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("personal status is missing %q:\n%s", fragment, got)
		}
	}
}

func TestFormatMiseStatusCompactsCurrentPackageAndRepoShapes(t *testing.T) {
	input := []byte(`{
  "packages": {"apt": {"available": true, "packages": [{
    "package": "curl", "requested_version": "latest",
    "installed_version": "8.0", "state": "installed"
  }]}},
  "repos": [{
    "path": "/home/test/.oh-my-zsh", "path_raw": "~/.oh-my-zsh",
    "url": "https://github.com/ohmyzsh/ohmyzsh.git",
    "origin": "https://github.com/ohmyzsh/ohmyzsh",
    "current_ref": "master",
    "current_sha": "1234567890abcdef1234567890abcdef12345678",
    "reason": "", "state": "current"
  }]
}`)

	got, err := formatMiseStatus(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Pacote (apt)", "curl", "Versão instalada: 8.0",
		"~/.oh-my-zsh", "Referência atual: master", "Commit atual: 1234567890ab", "atual",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("compact status is missing %q:\n%s", fragment, got)
		}
	}
	for _, unwanted := range []string{"Available", "Pacote · Apt · Pacote", "Origem atual", "1234567890abcdef"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("compact status contains %q:\n%s", unwanted, got)
		}
	}
}

func TestFormatMiseStatusLocalizesMissingDotfileSource(t *testing.T) {
	input := []byte(`{
  "dotfiles": {"files": [{
    "target": "~/.zshrc", "source": "home/.zshrc",
    "mode": "copy", "state": "source_missing"
  }]}
}`)

	got, err := formatMiseStatus(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "fonte ausente") {
		t.Fatalf("localized status = %q", got)
	}
}
