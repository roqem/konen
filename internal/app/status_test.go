package app

import (
	"bytes"
	"context"
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
		dir: stateDir, name: "/bin/mise",
		args: []string{"-C", stateDir, "bootstrap", "status", "--json"},
	}
	if len(runner.runs) != 2 || !reflect.DeepEqual(runner.runs[1], want) {
		t.Fatalf("status call = %#v, want %#v", runner.runs, want)
	}
	if !strings.Contains(out.String(), "Ferramenta") || !strings.Contains(out.String(), "go") {
		t.Fatalf("status output = %q", out.String())
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
		"Ferramenta", "codex", "Versão pedida: latest", "instalado",
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

func TestFormatMiseStatusHandlesEmptyState(t *testing.T) {
	got, err := formatMiseStatus([]byte(`{"tools": [], "packages": {}, "login_shell": null}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "Nenhum item declarado no estado.\n" {
		t.Fatalf("empty status = %q", got)
	}
}
