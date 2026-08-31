package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/config"
)

func TestParseRemoteSource(t *testing.T) {
	tests := []struct {
		input      string
		wantURL    string
		githubRepo string
	}{
		{"github:roqem/home", "https://github.com/roqem/home.git", "roqem/home"},
		{"https://github.com/roqem/home", "https://github.com/roqem/home.git", "roqem/home"},
		{"https://github.com/roqem/home.git", "https://github.com/roqem/home.git", "roqem/home"},
		{"git@github.com:roqem/home.git", "git@github.com:roqem/home.git", ""},
		{"https://git.example.com/roqem/home.git", "https://git.example.com/roqem/home.git", ""},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := parseRemoteSource(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.cloneURL != test.wantURL || got.githubRepository != test.githubRepo {
				t.Fatalf("parseRemoteSource() = %#v, want URL %q GitHub repo %q", got, test.wantURL, test.githubRepo)
			}
		})
	}
}

func TestParseRemoteSourceRejectsInvalidGitHubShorthand(t *testing.T) {
	for _, input := range []string{"github:", "github:owner", "github:owner/repo/extra", "github:owner/re po"} {
		if _, err := parseRemoteSource(input); err == nil {
			t.Errorf("parseRemoteSource(%q) should fail", input)
		}
	}
}

func TestInitRetriesPrivateGitHubCloneWithMiseProvidedCLI(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	cloneAttempts := 0
	authenticated := false
	runner := &fakeRunner{paths: map[string]string{"git": "/usr/bin/git", "mise": "/bin/mise"}}
	runner.runHook = func(call runCall) error {
		if call.name == "git" && len(call.args) > 0 && call.args[0] == "clone" {
			cloneAttempts++
			if cloneAttempts == 1 {
				return errors.New("authentication required")
			}
			if err := os.MkdirAll(stateDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\n"), 0o644)
		}
		if call.name == "/bin/mise" && containsArgs(call.args, "auth", "status") {
			return errors.New("not authenticated")
		}
		if call.name == "/bin/mise" && containsArgs(call.args, "auth", "login") {
			authenticated = true
		}
		return nil
	}
	runner.outputHook = func(call runCall) (string, error) {
		if call.name == "/bin/mise" && containsArgs(call.args, "repo", "view") {
			if !authenticated {
				return "", errors.New("not authenticated")
			}
			return `{"nameWithOwner":"roqem/home"}`, nil
		}
		return "", errors.New("unexpected output call")
	}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: configPath, HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{}, Interactive: true,
	})

	if err := application.Run(context.Background(), []string{"init", "--from", "github:roqem/home", stateDir}); err != nil {
		t.Fatal(err)
	}
	if cloneAttempts != 2 {
		t.Fatalf("clone attempts = %d, want 2", cloneAttempts)
	}
	for _, call := range runner.runs {
		if call.name == "git" && !reflect.DeepEqual(call.environment, []string{"GIT_TERMINAL_PROMPT=0"}) {
			t.Fatalf("assisted clone environment = %#v", call.environment)
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StateDir != stateDir {
		t.Fatalf("configured state = %q, want %q", cfg.StateDir, stateDir)
	}
	for _, fragment := range []string{
		"sem criar uma chave SSH",
		"mise baixará gh@latest sem sudo",
		"navegador pode estar em outro dispositivo",
		"credenciais HTTPS da conta ativa",
		"Revise o mise.toml",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("output is missing %q: %s", fragment, out.String())
		}
	}

	wantMiseCalls := [][]string{
		{"exec", "--yes", "--raw", "gh@latest", "--", "gh", "repo", "view", "roqem/home", "--json", "nameWithOwner"},
		{"exec", "--yes", "--raw", "gh@latest", "--", "gh", "auth", "status", "--hostname", "github.com"},
		{"exec", "--yes", "--raw", "gh@latest", "--", "gh", "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web"},
		{"exec", "--yes", "--raw", "gh@latest", "--", "gh", "repo", "view", "roqem/home", "--json", "nameWithOwner"},
		{"exec", "--yes", "--raw", "gh@latest", "--", "gh", "auth", "setup-git", "--hostname", "github.com"},
	}
	var gotMiseCalls [][]string
	for _, call := range runner.runs {
		if call.name == "/bin/mise" {
			gotMiseCalls = append(gotMiseCalls, call.args)
		}
	}
	if !reflect.DeepEqual(gotMiseCalls, wantMiseCalls) {
		t.Fatalf("mise GitHub calls = %#v, want %#v", gotMiseCalls, wantMiseCalls)
	}
}

func TestInitSwitchesAccountWhenActiveGitHubAccountCannotAccessRepository(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	cloneAttempts := 0
	switched := false
	runner := &fakeRunner{paths: map[string]string{"git": "/usr/bin/git", "gh": "/usr/bin/gh"}}
	runner.runHook = func(call runCall) error {
		switch {
		case call.name == "git" && len(call.args) > 0 && call.args[0] == "clone":
			cloneAttempts++
			if cloneAttempts == 1 {
				return errors.New("repository not found")
			}
			if err := os.MkdirAll(stateDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\n"), 0o644)
		case call.name == "/usr/bin/gh" && containsArgs(call.args, "auth", "switch"):
			switched = true
		}
		return nil
	}
	runner.outputHook = func(call runCall) (string, error) {
		if call.name == "/usr/bin/gh" && containsArgs(call.args, "repo", "view") {
			if !switched {
				return "", errors.New("active account has no access")
			}
			return `{"nameWithOwner":"roqem/home"}`, nil
		}
		return "", errors.New("unexpected output call")
	}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{}, Interactive: true,
	})

	if err := application.Run(context.Background(), []string{"init", "--from", "github:roqem/home", stateDir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "conta ativa do GitHub não acessa roqem/home") {
		t.Fatalf("account switch was not explained: %s", out.String())
	}
	for _, call := range runner.runs {
		if call.name == "/usr/bin/gh" && containsArgs(call.args, "auth", "login") {
			t.Fatalf("existing second account unexpectedly triggered login: %#v", runner.runs)
		}
	}
}

func TestInitPublicGitHubCloneDoesNotInvokeAuthentication(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"git": "/usr/bin/git", "mise": "/bin/mise"}}
	runner.runHook = func(call runCall) error {
		if call.name != "git" || len(call.args) == 0 || call.args[0] != "clone" {
			return fmt.Errorf("unexpected command: %#v", call)
		}
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\n"), 0o644)
	}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{}, Interactive: true,
	})

	if err := application.Run(context.Background(), []string{"init", "--from", "github:roqem/public-home", stateDir}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 1 || runner.runs[0].name != "git" {
		t.Fatalf("public clone unexpectedly authenticated: %#v", runner.runs)
	}
}

func TestPrivateGitHubFallbackRequiresInteractiveTerminal(t *testing.T) {
	root := t.TempDir()
	runner := &fakeRunner{
		paths: map[string]string{"git": "/usr/bin/git", "mise": "/bin/mise"},
		runHook: func(call runCall) error {
			if call.name == "git" {
				return errors.New("authentication required")
			}
			return nil
		},
	}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	err := application.Run(context.Background(), []string{"init", "--from", "github:roqem/home", filepath.Join(root, "state")})
	if err == nil || !strings.Contains(err.Error(), "terminal interativo") {
		t.Fatalf("non-interactive init error = %v", err)
	}
	if len(runner.runs) != 1 || runner.runs[0].name != "git" {
		t.Fatalf("non-interactive init unexpectedly ran auth: %#v", runner.runs)
	}
}

func containsArgs(args []string, want ...string) bool {
	for start := 0; start+len(want) <= len(args); start++ {
		if reflect.DeepEqual(args[start:start+len(want)], want) {
			return true
		}
	}
	return false
}
