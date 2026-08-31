package state

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/execx"
)

type fakeRunner struct {
	runs [][]string
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if name == "git" {
		return "/usr/bin/git", nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) error {
	f.runs = append(f.runs, append([]string{dir, name}, args...))
	return nil
}

func (f *fakeRunner) RunEnv(ctx context.Context, dir string, _ []string, name string, args ...string) error {
	return f.Run(ctx, dir, name, args...)
}

func (f *fakeRunner) Output(_ context.Context, _, _ string, _ ...string) (string, error) {
	return "", nil
}

func TestPrepareLocalCreatesPortableState(t *testing.T) {
	runner := &fakeRunner{}
	path := filepath.Join(t.TempDir(), "state")
	service := Service{Runner: runner}

	if err := service.PrepareLocal(context.Background(), path, true); err != nil {
		t.Fatalf("PrepareLocal() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(path, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `dotfiles.root = "home"`) {
		t.Fatalf("mise.toml does not use a portable relative dotfiles root:\n%s", data)
	}
	if !strings.Contains(string(data), `min_version = "2026.8.15"`) {
		t.Fatalf("mise.toml does not pin the minimum supported mise version:\n%s", data)
	}
	if !strings.Contains(string(data), `"~/.config/mise/config.toml" = { source = "mise.toml", mode = "symlink" }`) {
		t.Fatalf("mise.toml does not expose machine tools through the global mise config:\n%s", data)
	}
	ignore, err := os.ReadFile(filepath.Join(path, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, protected := range []string{"home/.git-credentials", "home/.config/gh/hosts.yml", "home/.netrc", "home/.ssh/id_*"} {
		if !strings.Contains(string(ignore), protected) {
			t.Errorf("default .gitignore does not protect %s:\n%s", protected, ignore)
		}
	}
	if _, err := os.Stat(filepath.Join(path, "home", ".gitkeep")); err != nil {
		t.Fatalf("home directory was not initialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "projects", ".gitkeep")); err != nil {
		t.Fatalf("projects directory was not initialized: %v", err)
	}
	if len(runner.runs) != 1 || runner.runs[0][1] != "git" {
		t.Fatalf("git init calls = %#v", runner.runs)
	}
}

func TestPrepareLocalRefusesUnrelatedNonEmptyDirectory(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "personal.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := Service{Runner: &fakeRunner{}}
	if err := service.PrepareLocal(context.Background(), path, false); err == nil {
		t.Fatal("PrepareLocal() should refuse a non-empty unrelated directory")
	}
}

func TestResolvePathExpandsHome(t *testing.T) {
	got, err := ResolvePath("~/machine", "/home/tester")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/home/tester/machine" {
		t.Fatalf("ResolvePath() = %q", got)
	}
}

func TestCloneWithoutPromptDisablesGitTerminalPrompts(t *testing.T) {
	runner := &environmentRunner{}
	path := filepath.Join(t.TempDir(), "state")
	service := Service{Runner: runner}

	if err := service.CloneWithoutPrompt(context.Background(), "https://github.com/example/state.git", path); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.environment, []string{"GIT_TERMINAL_PROMPT=0"}) {
		t.Fatalf("clone environment = %#v", runner.environment)
	}
}

func TestCloneWithCredentialHelperScopesAuthenticationToCloneAndRepository(t *testing.T) {
	runner := &environmentRunner{}
	path := filepath.Join(t.TempDir(), "state")
	service := Service{Runner: runner}
	helper := "!'/opt/mise' 'exec' '--raw' 'gh@latest' '--' 'gh' 'auth' 'git-credential'"

	if err := service.CloneWithCredentialHelper(context.Background(), "https://github.com/example/state.git", path, "https://github.com", helper); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.environment, []string{"GIT_TERMINAL_PROMPT=0"}) {
		t.Fatalf("clone environment = %#v", runner.environment)
	}
	want := [][]string{
		{"", "git", "-c", "credential.https://github.com.helper=", "-c", "credential.https://github.com.helper=" + helper, "clone", "--", "https://github.com/example/state.git", path},
		{path, "git", "config", "--local", "--replace-all", "credential.https://github.com.helper", ""},
		{path, "git", "config", "--local", "--add", "credential.https://github.com.helper", helper},
	}
	if !reflect.DeepEqual(runner.runs, want) {
		t.Fatalf("authenticated clone calls = %#v, want %#v", runner.runs, want)
	}
}

func TestCloneWithCredentialHelperUsingRealGitDoesNotChangeGlobalConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "state")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runner := execx.OSRunner{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	ctx := context.Background()
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, source, "git", "init", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "mise.toml"), []byte("[tools]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{
		{"add", "mise.toml"},
		{"-c", "user.name=Konen Test", "-c", "user.email=konen@example.invalid", "commit", "-m", "fixture"},
	} {
		if err := runner.Run(ctx, source, "git", command...); err != nil {
			t.Fatalf("git %v: %v", command, err)
		}
	}

	helper := "!'/opt/gh' 'auth' 'git-credential'"
	service := Service{Runner: runner}
	if err := service.CloneWithCredentialHelper(ctx, source, destination, "https://github.com", helper); err != nil {
		t.Fatal(err)
	}
	output, err := runner.Output(ctx, destination, "git", "config", "--local", "--get-all", "credential.https://github.com.helper")
	if err != nil {
		t.Fatal(err)
	}
	if output != "\n"+helper+"\n" {
		t.Fatalf("local helpers = %q", output)
	}
	for _, path := range []string{filepath.Join(home, ".gitconfig"), filepath.Join(home, ".config", "git", "config")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("global Git config was changed at %s: %v", path, err)
		}
	}
}

type environmentRunner struct {
	fakeRunner
	environment []string
}

func (r *environmentRunner) RunEnv(ctx context.Context, dir string, environment []string, name string, args ...string) error {
	r.environment = append([]string(nil), environment...)
	if err := os.MkdirAll(args[len(args)-1], 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(args[len(args)-1], "mise.toml"), []byte("[tools]\n"), 0o644); err != nil {
		return err
	}
	return r.Run(ctx, dir, name, args...)
}
