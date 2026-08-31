package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/ui"
)

func TestToolAddPreviewsWritesAndRefreshesTrust(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	runner.runHook = fakeMiseConfigSet
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: configPath, HomeDir: root, Out: &out, Err: &out,
		Runner: runner, Interactive: true,
		Prompter: toolPrompter{
			answer: ui.ToolAnswer{Name: "node", Version: "lts"}, confirmed: true,
		},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{"tool", "add"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[tools]\nnode = \"lts\"") {
		t.Fatalf("mise.toml does not contain tool:\n%s", data)
	}
	for _, fragment := range []string{
		"Alteração proposta", "+[tools]", "+node = \"lts\"",
		"Ferramenta adicionada ao estado: node@lts", "konen plan",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("output does not contain %q:\n%s", fragment, out.String())
		}
	}
	trusted, err := application.stateTrust().IsTrusted(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if !trusted {
		t.Fatal("guided tool mutation did not refresh local state trust")
	}

	var configCalls []runCall
	for _, call := range runner.runs {
		if containsArgs(call.args, "config", "set") {
			configCalls = append(configCalls, call)
		}
	}
	if len(configCalls) != 2 {
		t.Fatalf("config set calls = %#v, want preview and apply", configCalls)
	}
	if !containsEnvironment(configCalls[0].environment, "MISE_AUTO_INSTALL=false") ||
		!environmentHasPrefix(configCalls[0].environment, "MISE_TRUSTED_CONFIG_PATHS=") {
		t.Fatalf("preview environment = %#v", configCalls[0].environment)
	}
	if !containsEnvironment(configCalls[1].environment, "MISE_AUTO_INSTALL=false") {
		t.Fatalf("apply environment = %#v", configCalls[1].environment)
	}
}

func TestToolAddDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	runner.runHook = fakeMiseConfigSet
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config", "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	misePath := filepath.Join(stateDir, "mise.toml")
	before, err := os.ReadFile(misePath)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{"tool", "add", "--dry-run", "node", "lts"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(misePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("dry-run changed mise.toml:\n%s", after)
	}
	if !strings.Contains(out.String(), "Nenhuma alteração foi gravada.") {
		t.Fatalf("dry-run output = %q", out.String())
	}
}

func TestToolAddRequiresExplicitNonInteractiveWrite(t *testing.T) {
	application := New(Options{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	err := application.Run(context.Background(), []string{"tool", "add", "node", "lts"})
	if err == nil || !strings.Contains(err.Error(), "--dry-run") || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v", err)
	}
}

func TestToolAddSkipsSameVersionAndPreviewsUpdate(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	runner.runHook = fakeMiseConfigSet
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config", "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background(), []string{"tool", "add", "--yes", "node", "lts"}); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	runner.runs = nil
	if err := application.Run(context.Background(), []string{"tool", "add", "--yes", "node", "lts"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Ferramenta já declarada: node@lts") {
		t.Fatalf("same-version output = %q", out.String())
	}
	for _, call := range runner.runs {
		if containsArgs(call.args, "config", "set") {
			t.Fatalf("same version edited config: %#v", call)
		}
	}

	out.Reset()
	if err := application.Run(context.Background(), []string{"tool", "add", "--dry-run", "node", "22"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"-node = \"lts\"", "+node = \"22\""} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("update diff does not contain %q:\n%s", fragment, out.String())
		}
	}
}

func TestToolAddRejectsDottedName(t *testing.T) {
	err := validateToolAnswer(ui.ToolAnswer{Name: "vendor.tool", Version: "latest"})
	if err == nil || !strings.Contains(err.Error(), "nomes com ponto") {
		t.Fatalf("error = %v", err)
	}
}

func fakeMiseConfigSet(call runCall) error {
	if !containsArgs(call.args, "config", "set") {
		return nil
	}
	configPath := argumentAfter(call.args, "--file")
	if configPath == "" || len(call.args) < 2 {
		return errors.New("invalid fake config set call")
	}
	key := call.args[len(call.args)-2]
	version := call.args[len(call.args)-1]
	name := strings.TrimPrefix(key, "tools.")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	updated := updateFakeToolConfig(string(data), name, version)
	return os.WriteFile(configPath, []byte(updated), 0o600)
}

func updateFakeToolConfig(contents, name, version string) string {
	assignment := fmt.Sprintf("%s = %q", name, version)
	heading := "[tools]\n"
	headingIndex := strings.Index(contents, heading)
	if headingIndex < 0 {
		return strings.TrimRight(contents, "\n") + "\n\n" + heading + assignment + "\n"
	}
	sectionStart := headingIndex + len(heading)
	sectionEnd := len(contents)
	if next := strings.Index(contents[sectionStart:], "\n["); next >= 0 {
		sectionEnd = sectionStart + next + 1
	}
	section := contents[sectionStart:sectionEnd]
	prefix := name + " = "
	lines := strings.Split(section, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[index] = assignment
			return contents[:sectionStart] + strings.Join(lines, "\n") + contents[sectionEnd:]
		}
	}
	return contents[:sectionStart] + assignment + "\n" + contents[sectionStart:]
}

func argumentAfter(args []string, name string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
}

func containsEnvironment(environment []string, want string) bool {
	for _, value := range environment {
		if value == want {
			return true
		}
	}
	return false
}

func environmentHasPrefix(environment []string, prefix string) bool {
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
