package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
