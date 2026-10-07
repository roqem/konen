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
	"github.com/roqem/konen/internal/project"
	"github.com/roqem/konen/internal/ui"
)

type projectEditorPrompter struct {
	unusedPrompter
	edit    func(ui.ProjectAnswer, ui.ProjectServices) (ui.ProjectAnswer, error)
	choose  func(string, []ui.Choice) (string, error)
	confirm func(string) (bool, error)
}

func (p projectEditorPrompter) Project(answer ui.ProjectAnswer, services ui.ProjectServices) (ui.ProjectAnswer, error) {
	return p.edit(answer, services)
}

func (p projectEditorPrompter) Choose(title string, choices []ui.Choice) (string, error) {
	return p.choose(title, choices)
}

func (p projectEditorPrompter) Confirm(message string) (bool, error) {
	return p.confirm(message)
}

func TestTaskEditorStagesValidatesAndPreservesNativeFiles(t *testing.T) {
	for _, scenario := range []string{"save", "cancel", "invalid", "editor-error", "concurrent", "script", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "mise.toml")
			before := []byte("# keep comment\n[tools]\nnode = 'lts'\n[tasks.test]\nrun = 'old'\n")
			after := bytes.Replace(before, []byte("'old'"), []byte("'new'"), 1)
			mode := os.FileMode(0o644)
			task := project.Task{Name: "test", Source: path}
			if scenario == "script" {
				path = filepath.Join(dir, "test.sh")
				before, after = []byte("#!/bin/sh\necho old\n"), []byte("#!/bin/sh\necho new\n")
				mode, task.File = 0o755, path
			}
			if scenario == "invalid" {
				after = []byte("[tasks.test\n")
			}
			if err := os.WriteFile(path, before, mode); err != nil {
				t.Fatal(err)
			}
			if scenario == "symlink" {
				link := filepath.Join(dir, "global.toml")
				if err := os.Symlink(path, link); err != nil {
					t.Fatal(err)
				}
				task.Source = link
			}
			var draft string
			var out bytes.Buffer
			runner := &fakeRunner{paths: map[string]string{"nvim": "/bin/nvim"}}
			runner.runHook = func(call runCall) error {
				if call.name != "/bin/nvim" || call.dir != dir || len(call.args) != 3 || call.args[1] != "--" {
					t.Fatalf("editor call = %#v", call)
				}
				if scenario != "script" && call.args[0] != "+4" {
					t.Fatalf("editor should open at the task: %v", call.args)
				}
				draft = call.args[2]
				if draft == path {
					t.Fatal("editor received the live file")
				}
				if scenario == "editor-error" {
					return errors.New("cancelled with :cq")
				}
				return os.WriteFile(draft, after, 0o600)
			}
			prompter := projectEditorPrompter{confirm: func(message string) (bool, error) {
				if scenario == "concurrent" {
					if err := os.WriteFile(path, []byte("# concurrent edit\n"), mode); err != nil {
						t.Fatal(err)
					}
				}
				return scenario != "cancel" && scenario != "invalid", nil
			}}
			application := New(Options{HomeDir: dir, Runner: runner, Prompter: prompter, Out: &out, Err: &out})
			err := application.editProjectTask(context.Background(), dir, task)
			if scenario == "editor-error" || scenario == "concurrent" {
				if err == nil {
					t.Fatal("expected editor/conflict error")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			want := before
			if scenario == "save" || scenario == "script" || scenario == "symlink" {
				want = after
			} else if scenario == "concurrent" {
				want = []byte("# concurrent edit\n")
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("file = %q, want %q (error %v)", got, want, err)
			}
			info, _ := os.Stat(path)
			if info.Mode().Perm() != mode {
				t.Fatalf("mode = %v, want %v", info.Mode(), mode)
			}
			if _, err := os.Stat(draft); !os.IsNotExist(err) {
				t.Fatalf("draft was not removed: %v", err)
			}
			if len(runner.runs) != 1 {
				t.Fatalf("unexpected task execution or approval: %#v", runner.runs)
			}
		})
	}
}

func TestProjectTasksListAndSelectNativeTaskWithoutExecutingIt(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "nvim": "/bin/nvim"}}
	application := New(Options{ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		WorkDir: root, Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{}})
	stateDir := filepath.Join(root, "state")
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: root}
	if _, err := store.Save("sample", project.Manifest{Version: 2, Path: root, Tabs: []project.Tab{{Title: "Terminal"}}}); err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	runner.outputHook = func(call runCall) (string, error) {
		if call.dir != root || !reflect.DeepEqual(call.args, []string{"tasks", "ls", "--json"}) {
			t.Fatalf("task query = %#v", call)
		}
		return `[{"name":"test","description":"Run tests","source":"mise.toml","run":["go test ./..."]}]`, nil
	}
	if err := application.Run(context.Background(), []string{"project", "tasks"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"test", "Run tests", "go test ./...", "mise.toml"} {
		assertOutputContains(t, out.String(), fragment)
	}
	if len(runner.runs) != 1 {
		t.Fatalf("list must only query mise: %#v", runner.runs)
	}
	application.options.Interactive = true
	path := filepath.Join(root, "mise.toml")
	if err := os.WriteFile(path, []byte("[tasks.test]\nrun = 'true'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	application.options.Prompter = projectEditorPrompter{choose: func(_ string, choices []ui.Choice) (string, error) {
		if len(choices) != 1 || choices[0].Value != "test" {
			t.Fatalf("task choices = %#v", choices)
		}
		return "test", nil
	}}
	if err := application.Run(context.Background(), []string{"project", "task", "edit", "sample"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 3 || runner.runs[2].name != "/bin/nvim" {
		t.Fatalf("unexpected execution: %#v", runner.runs)
	}
}

func TestProjectEditInfersDirectoryAndPreservesTabOrderOnSaveOrCancel(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "cancel"}[cancel], func(t *testing.T) {
			root := t.TempDir()
			var out bytes.Buffer
			runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise"}}
			application := New(Options{ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
				WorkDir: root, Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{}, Interactive: true})
			stateDir := filepath.Join(root, "state")
			if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
				t.Fatal(err)
			}
			store := project.Store{StateDir: stateDir, HomeDir: root}
			path, err := store.Save("sample", project.Manifest{Version: 2, Path: root,
				Tabs: []project.Tab{{Title: "Claude", Command: "claude"}, {Title: "Terminal"}}})
			if err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			application.options.Prompter = projectEditorPrompter{edit: func(answer ui.ProjectAnswer, _ ui.ProjectServices) (ui.ProjectAnswer, error) {
				if answer.Name != "sample" || !answer.Tabs[0].Hold || !answer.Tabs[1].Hold {
					t.Fatalf("edit defaults = %#v", answer)
				}
				answer.Tabs[0], answer.Tabs[1] = answer.Tabs[1], answer.Tabs[0]
				answer.Tabs[1].Command = "codex"
				if cancel {
					return answer, huh.ErrUserAborted
				}
				return answer, nil
			}}
			if err := application.Run(context.Background(), []string{"project", "edit"}); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			if cancel {
				if !bytes.Equal(before, after) {
					t.Fatal("cancelling changed the manifest")
				}
				return
			}
			manifest, _, err := store.Load("sample")
			if err != nil || manifest.Tabs[0].Title != "Terminal" || manifest.Tabs[1].Command != "codex" {
				t.Fatalf("updated manifest = %#v, error %v", manifest, err)
			}
			trusted, err := application.projectTrust().IsTrusted(path)
			if err != nil || !trusted {
				t.Fatalf("saved project trust = %v, %v", trusted, err)
			}
		})
	}
}

func TestDevSingleTabKeepsInvokingTabAndTrustsEntireManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	runner := &fakeRunner{paths: map[string]string{"mise": "/bin/mise", "kitten": "/bin/kitten"}, outputs: map[string]string{"/bin/kitten": "42"}}
	application := New(Options{ConfigPath: filepath.Join(root, "config.toml"), HomeDir: root,
		WorkDir: root, Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
		Getenv: func(key string) string {
			if key == "KITTY_WINDOW_ID" {
				return "1"
			}
			return ""
		}})
	stateDir := filepath.Join(root, "state")
	if err := application.Run(context.Background(), []string{"init", stateDir}); err != nil {
		t.Fatal(err)
	}
	store := project.Store{StateDir: stateDir, HomeDir: root}
	path, err := store.Save("sample", project.Manifest{Version: 2, Path: root, KeepInvokingTab: boolPointer(false),
		Tabs: []project.Tab{{Title: "Claude", Command: "claude"}, {Title: "Terminal"}}})
	if err != nil {
		t.Fatal(err)
	}
	runner.runs = nil
	args := []string{"dev", "sample", "--tab", "Terminal"}
	if err := application.Run(context.Background(), args); err == nil || !strings.Contains(err.Error(), "aprovados") {
		t.Fatalf("untrusted tab error = %v", err)
	}
	if len(runner.runs) != 0 {
		t.Fatal("untrusted tab reached Kitty")
	}
	if err := application.projectTrust().Trust(path); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 3 {
		t.Fatalf("single tab calls = %#v", runner.runs)
	}
	launch := strings.Join(runner.runs[1].args, " ")
	if !strings.Contains(launch, "--tab-title Terminal") || !strings.Contains(launch, "--hold") || strings.Contains(launch, "claude") {
		t.Fatalf("single tab launch = %s", launch)
	}
	runner.runs = nil
	out.Reset()
	if err := application.Run(context.Background(), []string{"dev", "sample", "--tab=Terminal", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 0 || strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "voltar ao shell") {
		t.Fatalf("single tab preview executed or included other tabs: %q, %#v", out.String(), runner.runs)
	}
	for _, args := range [][]string{{"dev", "sample", "--tab"}, {"dev", "sample", "--tab="}, {"dev", "sample", "--tab", "missing"}} {
		if err := application.Run(context.Background(), args); err == nil {
			t.Fatalf("invalid tab accepted: %v", args)
		}
	}
	if len(runner.runs) != 0 {
		t.Fatalf("invalid tab launched a session: %#v", runner.runs)
	}
}
