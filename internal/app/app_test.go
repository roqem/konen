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

	"charm.land/huh/v2"
	"github.com/roqem/konen/internal/execx"
	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/ui"
)

type runCall struct {
	dir         string
	environment []string
	name        string
	args        []string
}

type fakeRunner struct {
	paths      map[string]string
	outputs    map[string]string
	runs       []runCall
	runHook    func(runCall) error
	outputHook func(runCall) (string, error)
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if path, ok := f.paths[name]; ok {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) error {
	call := runCall{dir: dir, name: name, args: append([]string(nil), args...)}
	f.runs = append(f.runs, call)
	if f.runHook != nil {
		return f.runHook(call)
	}
	return nil
}

func (f *fakeRunner) RunEnv(_ context.Context, dir string, environment []string, name string, args ...string) error {
	call := runCall{
		dir: dir, environment: append([]string(nil), environment...),
		name: name, args: append([]string(nil), args...),
	}
	f.runs = append(f.runs, call)
	if f.runHook != nil {
		return f.runHook(call)
	}
	return nil
}

func (f *fakeRunner) Output(_ context.Context, dir, name string, args ...string) (string, error) {
	return f.output(runCall{dir: dir, name: name, args: append([]string(nil), args...)})
}

func (f *fakeRunner) OutputEnv(
	_ context.Context,
	dir string,
	environment []string,
	name string,
	args ...string,
) (string, error) {
	return f.output(runCall{
		dir: dir, environment: append([]string(nil), environment...),
		name: name, args: append([]string(nil), args...),
	})
}

func (f *fakeRunner) output(call runCall) (string, error) {
	f.runs = append(f.runs, call)
	if f.outputHook != nil {
		return f.outputHook(call)
	}
	if output, ok := f.outputs[call.name]; ok {
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
func (unusedPrompter) Tool(ui.ToolAnswer) (ui.ToolAnswer, error) {
	return ui.ToolAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) Package(ui.PackageAnswer) (ui.PackageAnswer, error) {
	return ui.PackageAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) Repository(ui.RepositoryAnswer) (ui.RepositoryAnswer, error) {
	return ui.RepositoryAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) PersonalCommand(ui.PersonalCommandAnswer) (ui.PersonalCommandAnswer, error) {
	return ui.PersonalCommandAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) PersonalInstaller(ui.PersonalInstallerAnswer) (ui.PersonalInstallerAnswer, error) {
	return ui.PersonalInstallerAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) ChooseApplyParts([]ui.ApplyPart) ([]string, error) {
	return nil, errors.New("unexpected prompt")
}
func (unusedPrompter) Confirm(string) (bool, error) { return false, errors.New("unexpected prompt") }
func (unusedPrompter) Project(ui.ProjectAnswer) (ui.ProjectAnswer, error) {
	return ui.ProjectAnswer{}, errors.New("unexpected prompt")
}
func (unusedPrompter) ChooseProject([]string) (string, error) {
	return "", errors.New("unexpected prompt")
}

type projectPrompter struct {
	unusedPrompter
	answer ui.ProjectAnswer
}

type menuPrompter struct {
	unusedPrompter
	action string
	err    error
}

type toolPrompter struct {
	unusedPrompter
	answer    ui.ToolAnswer
	confirmed bool
}

func (p menuPrompter) Menu(bool) (string, error) {
	return p.action, p.err
}

func (p projectPrompter) Project(ui.ProjectAnswer) (ui.ProjectAnswer, error) {
	return p.answer, nil
}

func (p toolPrompter) Tool(ui.ToolAnswer) (ui.ToolAnswer, error) {
	return p.answer, nil
}

func (p toolPrompter) Confirm(string) (bool, error) {
	return p.confirmed, nil
}

var _ execx.Runner = (*fakeRunner)(nil)

func TestInitAndApply(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	applied := false
	runner := &fakeRunner{
		paths: map[string]string{"mise": "/bin/mise", "git": "/bin/git"},
		outputHook: func(call runCall) (string, error) {
			if strings.Join(call.args, " ") == "-C "+stateDir+" bootstrap status --json" {
				state := "missing"
				installed := "false"
				if applied {
					state = "installed"
					installed = "true"
				}
				return `{"tools":[{"tool":"go","state":"` + state + `","installed":` + installed + `}]}`, nil
			}
			return "", errors.New("unexpected output call")
		},
		runHook: func(call runCall) error {
			if strings.Contains(" "+strings.Join(call.args, " ")+" ", " bootstrap --yes ") {
				applied = true
			}
			return nil
		},
	}
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
		dir:         stateDir,
		environment: miseStateEnvironment(stateDir),
		name:        "/bin/mise",
		args:        []string{"-C", stateDir, "bootstrap", "--yes"},
	}
	if len(runner.runs) != 5 || !reflect.DeepEqual(runner.runs[3], want) {
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
	if !strings.Contains(out.String(), "O Konen não cria commits") {
		t.Fatalf("init output does not explain Git ownership: %q", out.String())
	}
	for _, fragment := range []string{"Resumo do apply", "Convergiram nesta execução", "Ferramenta: 1"} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("apply output is missing %q: %s", fragment, out.String())
		}
	}
}

func TestApplyDryRunPinsTheSelectedState(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &out, Err: &out,
		Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{"apply", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "não é confiável") {
		t.Fatalf("isolated dry-run still warns about another global config: %q", out.String())
	}
	want := runCall{
		dir:         stateDir,
		environment: miseStateEnvironment(stateDir),
		name:        "/bin/mise",
		args:        []string{"-C", stateDir, "bootstrap", "--dry-run"},
	}
	if len(runner.runs) != 1 || !reflect.DeepEqual(runner.runs[0], want) {
		t.Fatalf("runs = %#v, want %#v", runner.runs, want)
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

func TestDotfileAddPinsTheStateConfig(t *testing.T) {
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
	if err := application.Run(context.Background(), []string{"dotfile", "add", "--mode", "copy", "/tmp/example"}); err != nil {
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

func TestDotfileAddResolvesRelativePathFromWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workspace")
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, WorkDir: workDir,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background(), []string{"dotfile", "add", "config/example.toml"}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(workDir, "config", "example.toml")
	if got := runner.runs[1].args[len(runner.runs[1].args)-1]; got != want {
		t.Fatalf("relative target = %q, want %q", got, want)
	}
}

func TestDotfileAddRefusesMachineSpecificGitAndCredentialFiles(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		want string
	}{
		{"~/.gitconfig", "~/.config/git/config"},
		{"~/.git-credentials", "contém credenciais"},
		{"~/.config/gh/hosts.yml", "contém credenciais"},
		{"~/.netrc", "contém credenciais"},
		{"~/.ssh", "contém credenciais"},
		{"~/.ssh/id_ed25519", "chave SSH privada"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			err := application.Run(context.Background(), []string{"dotfile", "add", test.path})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("dotfile add error = %v", err)
			}
		})
	}
	if len(runner.runs) != 1 {
		t.Fatalf("protected captures invoked mise: %#v", runner.runs)
	}
}

func TestLegacyAddExplainsProjectAndDotfileCommands(t *testing.T) {
	application := New(Options{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	err := application.Run(context.Background(), []string{"add", "somewhere"})
	if err == nil {
		t.Fatal("ambiguous add should fail")
	}
	for _, fragment := range []string{"é ambíguo", "konen project add", "konen dotfile add"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("add error is missing %q: %v", fragment, err)
		}
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

func TestPlanUsesFullBootstrapDryRun(t *testing.T) {
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
	if err := application.Run(context.Background(), []string{"plan"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"-C", stateDir, "bootstrap", "--dry-run"}
	if len(runner.runs) != 2 || !reflect.DeepEqual(runner.runs[1].args, want) {
		t.Fatalf("mise args = %#v, want %#v", runner.runs, want)
	}
}

func TestProjectsListsThroughPluralCommand(t *testing.T) {
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
	if _, err := store.Save("sample", project.Manifest{
		Version: 1, Path: root, Tabs: []project.Tab{{Title: "Terminal"}},
	}); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{"projects"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"Projeto", "Aprovação", "Pasta", "sample"} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("projects output is missing %q: %s", fragment, out.String())
		}
	}
}

func TestRegisteredProjectNameIsDevShortcut(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "sample")
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root, WorkDir: root,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: root}
	if _, err := store.Save("sample", project.Manifest{
		Version: 1, Path: projectDir, Tabs: []project.Tab{{Title: "Terminal"}},
	}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	runner.runs = nil

	if err := application.Run(context.Background(), []string{"sample", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	assertOutputContains(t, out.String(), "Projeto: sample")
	assertOutputContains(t, out.String(), "Aprovação local: pendente")
	if len(runner.runs) != 0 {
		t.Fatalf("shortcut dry-run launched commands: %#v", runner.runs)
	}
}

func TestUnknownProjectShortcutGivesFocusedGuidance(t *testing.T) {
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
	out.Reset()

	err := application.Run(context.Background(), []string{"unknown-app"})
	if err == nil {
		t.Fatal("unknown shortcut should fail")
	}
	for _, fragment := range []string{"não é um comando nem um projeto cadastrado", "konen projects", "konen project add"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("shortcut error is missing %q: %v", fragment, err)
		}
	}
	if out.Len() != 0 {
		t.Fatalf("unknown shortcut printed the full help unexpectedly: %q", out.String())
	}
}

func TestInteractiveExitAndAbortAreClean(t *testing.T) {
	for _, test := range []struct {
		name     string
		prompter ui.Prompter
	}{
		{name: "exit option", prompter: menuPrompter{action: "__exit"}},
		{name: "keyboard abort", prompter: menuPrompter{err: huh.ErrUserAborted}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			application := New(Options{
				ConfigPath: filepath.Join(root, "missing.toml"), HomeDir: root,
				Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: &fakeRunner{},
				Prompter: test.prompter, Interactive: true,
			})
			if err := application.Run(context.Background(), nil); err != nil {
				t.Fatalf("interactive exit returned error: %v", err)
			}
		})
	}
}

func TestHelpGroupsCommandsAndSeparatesAddOperations(t *testing.T) {
	var out bytes.Buffer
	application := New(Options{Out: &out})
	if err := application.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Início rápido:", "Máquina:", "Estado:", "Projetos:", "Shell:",
		"konen NOME", "konen tool add [NOME] [VERSÃO]",
		"konen command add [NOME]",
		"konen installer add [NOME]",
		"konen project add [DIR]", "konen dotfile add CAMINHO...",
	} {
		assertOutputContains(t, out.String(), fragment)
	}
	if strings.Contains(out.String(), "\n  konen add ") {
		t.Fatalf("help still advertises ambiguous add:\n%s", out.String())
	}
}

func TestProjectAddGuidedFlowSavesAndTrustsManifest(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	stateDir := filepath.Join(root, "state")
	projectDir := filepath.Join(home, "Projects", "sample")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath:  filepath.Join(root, "config", "config.toml"),
		HomeDir:     home,
		WorkDir:     projectDir,
		Out:         &out,
		Err:         &out,
		Runner:      runner,
		Interactive: true,
		Prompter: projectPrompter{answer: ui.ProjectAnswer{
			Name: "sample", Path: projectDir, KeepInvokingTab: true,
			Tabs: []ui.ProjectTabAnswer{{Title: "Terminal", Command: "git status", Hold: true}},
		}},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	out.Reset()

	if err := application.Run(context.Background(), []string{"project", "add"}); err != nil {
		t.Fatal(err)
	}
	assertOutputContains(t, out.String(), "Projeto cadastrado e aprovado: sample")

	store := project.Store{StateDir: stateDir, HomeDir: home}
	manifest, manifestPath, err := store.Load("sample")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Path != "~/Projects/sample" || len(manifest.Tabs) != 1 || manifest.Tabs[0].Command != "git status" {
		t.Fatalf("saved manifest = %#v", manifest)
	}
	trusted, err := application.projectTrust().IsTrusted(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !trusted {
		t.Fatal("manifest created by the guided flow was not trusted")
	}

	out.Reset()
	if err := application.Run(context.Background(), []string{"projects"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"sample", "aprovado", "~/Projects/sample"} {
		assertOutputContains(t, out.String(), fragment)
	}
}

func assertOutputContains(t *testing.T, output, fragment string) {
	t.Helper()
	if !strings.Contains(output, fragment) {
		t.Fatalf("output does not contain %q: %s", fragment, output)
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
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"),
		HomeDir:    root,
		Out:        &out,
		Err:        &out,
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
	for _, fragment := range []string{"Superfície executável aprovada", "mise.toml"} {
		assertOutputContains(t, out.String(), fragment)
	}
}

func TestChangedInstallerRevokesStateApprovalBeforeMiseRuns(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}, outputs: map[string]string{
		"/bin/mise": `{"tools":[]}`,
	}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	installer := filepath.Join(stateDir, "mise-tasks", "install", "sample")
	if err := os.WriteFile(installer, []byte("#!/bin/sh\nprintf changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil

	err := application.Run(context.Background(), []string{"status"})
	if err == nil || !strings.Contains(err.Error(), "instalador") || !strings.Contains(err.Error(), "konen trust") {
		t.Fatalf("status error = %v", err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("mise ran before Konen approval: %#v", runner.runs)
	}
}

func TestTrustRejectsSymlinkedExecutableSurfaceBeforeMiseRuns(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(filepath.Join(stateDir, "scripts", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside-command")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(stateDir, "scripts", "bin", "unsafe")); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
	application := New(Options{
		ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: runner, Prompter: unusedPrompter{},
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}

	err := application.Run(context.Background(), []string{"trust"})
	if err == nil || !strings.Contains(err.Error(), "não aceita links") {
		t.Fatalf("trust error = %v", err)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("mise trusted unsafe state before validation: %#v", runner.runs)
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

func TestDevDryRunShowsPendingApprovalWithoutLaunching(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	projectDir := filepath.Join(home, "project")
	stateDir := filepath.Join(root, "state")
	configPath := filepath.Join(root, "config", "config.toml")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "kitten": "/bin/kitten"}}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: configPath, HomeDir: home, WorkDir: projectDir,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
		Getenv: func(string) string { return "4" },
	})
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: home}
	if _, err := store.Save("sample", project.Manifest{
		Version: 1, Path: projectDir, Tabs: []project.Tab{{Title: "Terminal"}},
	}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	out.Reset()

	if err := application.Run(context.Background(), []string{"dev", "sample", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Aprovação local: pendente") {
		t.Fatalf("dry-run output = %q", out.String())
	}
	if len(runner.runs) != 0 {
		t.Fatalf("dry-run launched commands: %#v", runner.runs)
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
