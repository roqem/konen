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

	"github.com/roqem/konen/internal/execx"
	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/ui"
)

type runCall struct {
	dir  string
	name string
	args []string
}

type fakeRunner struct {
	paths   map[string]string
	outputs map[string]string
	runs    []runCall
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if path, ok := f.paths[name]; ok {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) error {
	f.runs = append(f.runs, runCall{dir: dir, name: name, args: append([]string(nil), args...)})
	return nil
}

func (f *fakeRunner) Output(_ context.Context, dir, name string, args ...string) (string, error) {
	f.runs = append(f.runs, runCall{dir: dir, name: name, args: append([]string(nil), args...)})
	if output, ok := f.outputs[name]; ok {
		return output, nil
	}
	return "", errors.New("no output")
}

type unusedPrompter struct{}

func (unusedPrompter) Menu(bool) (string, error) { return "", errors.New("unexpected prompt") }
func (unusedPrompter) Init(string) (ui.InitAnswer, error) {
	return ui.InitAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) AddTarget() (string, error) { return "", errors.New("unexpected prompt") }
func (unusedPrompter) Project(ui.ProjectAnswer) (ui.ProjectAnswer, error) {
	return ui.ProjectAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) ChooseProject([]string) (string, error) {
	return "", errors.New("unexpected prompt")
}

var _ execx.Runner = (*fakeRunner)(nil)

func TestInitAndApply(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "git": "/bin/git"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: configPath,
		HomeDir:    root,
		Out:        &out,
		Err:        &out,
		Runner:     runner,
		Prompter:   unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"init", "--git", stateDir}); err != nil {
		t.Fatalf("init error = %v", err)
	}
	if err := application.Run(context.Background(), []string{"apply", "--yes"}); err != nil {
		t.Fatalf("apply error = %v", err)
	}

	want := runCall{
		dir:  stateDir,
		name: "/bin/mise",
		args: []string{"-C", stateDir, "bootstrap", "--yes"},
	}
	if len(runner.runs) != 3 || !reflect.DeepEqual(runner.runs[2], want) {
		t.Fatalf("runs = %#v, want final call %#v", runner.runs, want)
	}
	wantTrust := runCall{
		dir:  stateDir,
		name: "/bin/mise",
		args: []string{"trust", filepath.Join(stateDir, "mise.toml")},
	}
	if !reflect.DeepEqual(runner.runs[1], wantTrust) {
		t.Fatalf("runs = %#v, want trust call %#v", runner.runs, wantTrust)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "mise.toml")); err != nil {
		t.Fatalf("mise.toml missing: %v", err)
	}
}

func TestStatusRequiresMise(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config.toml")
	runner := &fakeRunner{paths: map[string]string{}}
	application := New(Options{
		ConfigPath: configPath,
		HomeDir:    root,
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
		Runner:     runner,
		Prompter:   unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background(), []string{"status"}); err == nil {
		t.Fatal("status should fail when mise is unavailable")
	}
}

func TestAddPinsTheStateConfig(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config.toml")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: configPath,
		HomeDir:    root,
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
		Runner:     runner,
		Prompter:   unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background(), []string{"add", "--mode", "copy", "/tmp/example"}); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"-C", stateDir,
		"bootstrap", "dotfiles", "add",
		"--path", filepath.Join(stateDir, "mise.toml"),
		"--mode", "copy",
		"/tmp/example",
	}
	if len(runner.runs) != 2 || !reflect.DeepEqual(runner.runs[1].args, want) {
		t.Fatalf("mise args = %#v, want %#v", runner.runs, want)
	}
}

func TestDiffUsesNativeMiseDiff(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root,
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
		Runner:     runner,
		Prompter:   unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background(), []string{"diff"}); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"-C", stateDir,
		"bootstrap", "dotfiles", "diff",
	}
	if len(runner.runs) != 2 || !reflect.DeepEqual(runner.runs[1].args, want) {
		t.Fatalf("mise args = %#v, want %#v", runner.runs, want)
	}
}

func TestExistingStateNeedsExplicitTrust(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("min_version = \"2026.8.15\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root,
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
		Runner:     runner,
		Prompter:   unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("existing state was trusted implicitly: %#v", runner.runs)
	}
	if err := application.Run(context.Background(), []string{"trust"}); err != nil {
		t.Fatal(err)
	}
	want := runCall{
		dir:  stateDir,
		name: "/bin/mise",
		args: []string{"trust", filepath.Join(stateDir, "mise.toml")},
	}
	if len(runner.runs) != 1 || !reflect.DeepEqual(runner.runs[0], want) {
		t.Fatalf("runs = %#v, want %#v", runner.runs, want)
	}
}

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		got  string
		want bool
	}{
		{"2026.8.15", true},
		{"2026.9.0", true},
		{"2027.1.0", true},
		{"2026.8.13", false},
		{"2025.12.99", false},
	}
	for _, test := range tests {
		if got := versionAtLeast(test.got, minimumMiseVersion); got != test.want {
			t.Errorf("versionAtLeast(%q) = %v, want %v", test.got, got, test.want)
		}
	}

	if got, err := extractVersion("2026.8.15 linux-x64 (2026-08-26)"); err != nil || got != "2026.8.15" {
		t.Fatalf("extractVersion() = %q, %v", got, err)
	}
}

func TestFindCommandPrefersSiblingBinary(t *testing.T) {
	binDir := t.TempDir()
	misePath := filepath.Join(binDir, "mise")
	if err := os.WriteFile(misePath, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	application := New(Options{
		BinDir: binDir,
		Runner: &fakeRunner{paths: map[string]string{"mise": "/usr/bin/mise"}},
	})

	got, err := application.findCommand("mise")
	if err != nil {
		t.Fatal(err)
	}
	if got != misePath {
		t.Fatalf("findCommand() = %q, want %q", got, misePath)
	}
}

func TestDevOpensTabsInCurrentKittyAndFocusesFirst(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	projectDir := filepath.Join(home, "Documents", "Projects", "sample")
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{
		paths:   map[string]string{"mise": "/bin/mise", "kitten": "/bin/kitten"},
		outputs: map[string]string{"/bin/kitten": "41\n"},
	}
	application := New(Options{
		ConfigPath: configPath,
		HomeDir:    home,
		WorkDir:    projectDir,
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
		Runner:     runner,
		Prompter:   unusedPrompter{},
		Getenv: func(name string) string {
			switch name {
			case "KITTY_WINDOW_ID":
				return "4"
			case "SHELL":
				return "/bin/zsh"
			default:
				return ""
			}
		},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: home}
	manifestPath, err := store.Save("sample", project.Manifest{
		Version: 1,
		Path:    "~/Documents/Projects/sample",
		Tabs: []project.Tab{
			{Title: "Editor", Command: "nvim ."},
			{Title: "Status", Command: "git status", Hold: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.projectTrust().Trust(manifestPath); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	if err := application.Run(context.Background(), []string{"dev"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 4 {
		t.Fatalf("runs = %#v", runner.runs)
	}
	wantFirst := runCall{
		dir: projectDir, name: "/bin/kitten",
		args: []string{"@", "launch", "--self", "--type=tab", "--keep-focus", "--tab-title", "Editor", "--cwd", projectDir, "--add-to-session", "konen-sample", "/bin/zsh", "-lic", "nvim ."},
	}
	if !reflect.DeepEqual(runner.runs[1], wantFirst) {
		t.Fatalf("first launch = %#v, want %#v", runner.runs[1], wantFirst)
	}
	wantFocus := runCall{dir: projectDir, name: "/bin/kitten", args: []string{"@", "focus-tab", "--match", "window_id:41"}}
	if !reflect.DeepEqual(runner.runs[3], wantFocus) {
		t.Fatalf("focus = %#v, want %#v", runner.runs[3], wantFocus)
	}
}

func TestDevRefusesChangedProjectUntilTrustedAgain(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	projectDir := filepath.Join(home, "project")
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "kitten": "/bin/kitten"}}
	application := New(Options{
		ConfigPath: configPath, HomeDir: home, WorkDir: projectDir,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
		Getenv: func(string) string { return "4" },
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: home}
	manifestPath, err := store.Save("sample", project.Manifest{
		Version: 1, Path: projectDir, Tabs: []project.Tab{{Title: "Terminal"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.projectTrust().Trust(manifestPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("version = 1\npath = '"+projectDir+"'\n[[tabs]]\ntitle = 'Changed'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	if err := application.Run(context.Background(), []string{"dev", "sample"}); err == nil || !strings.Contains(err.Error(), "não foram aprovados") {
		t.Fatalf("dev error = %v", err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("commands ran before trust: %#v", runner.runs)
	}
}

func TestRenderKittySessionQuotesCommands(t *testing.T) {
	got := renderKittySession("/tmp/project with space", "/bin/zsh", project.Manifest{
		Tabs: []project.Tab{{Title: "It's ready", Command: "printf '%s' ok", Hold: true}},
	})
	want := "new_tab 'It'\\''s ready'\n" +
		"cd '/tmp/project with space'\n" +
		"launch --hold '/bin/zsh' -lic 'printf '\\''%s'\\'' ok'\n\n" +
		"focus_tab 0\n"
	if got != want {
		t.Fatalf("session =\n%s\nwant =\n%s", got, want)
	}
}

func TestDevCanCloseInvokingKittyWindow(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	projectDir := filepath.Join(home, "project")
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{
		paths:   map[string]string{"mise": "/bin/mise", "kitten": "/bin/kitten"},
		outputs: map[string]string{"/bin/kitten": "51\n"},
	}
	application := New(Options{
		ConfigPath: configPath, HomeDir: home, WorkDir: projectDir,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
		Getenv: func(name string) string {
			if name == "KITTY_WINDOW_ID" {
				return "4"
			}
			if name == "SHELL" {
				return "/bin/zsh"
			}
			return ""
		},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	keep := false
	store := project.Store{StateDir: stateDir, HomeDir: home}
	manifestPath, err := store.Save("sample", project.Manifest{
		Version: 1, Path: projectDir, KeepInvokingTab: &keep,
		Tabs: []project.Tab{{Title: "Terminal"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.projectTrust().Trust(manifestPath); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	if err := application.Run(context.Background(), []string{"dev", "sample"}); err != nil {
		t.Fatal(err)
	}
	want := runCall{
		dir: projectDir, name: "/bin/kitten",
		args: []string{"@", "close-window", "--self", "--no-response"},
	}
	if len(runner.runs) != 4 || !reflect.DeepEqual(runner.runs[3], want) {
		t.Fatalf("calls = %#v, want final call %#v", runner.runs, want)
	}
}
