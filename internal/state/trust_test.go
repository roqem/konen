package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExecutionDigestTracksConfigTasksCommandsAndModes(t *testing.T) {
	root := t.TempDir()
	writeTrustFixture(t, filepath.Join(root, "mise.toml"), "[tools]\n", 0o644)
	writeTrustFixture(t, filepath.Join(root, "mise-tasks", "install", "docker"), "#!/bin/sh\n", 0o755)
	writeTrustFixture(t, filepath.Join(root, "scripts", "bin", "hello"), "#!/bin/sh\n", 0o755)
	writeTrustFixture(t, filepath.Join(root, "home", ".zshrc"), "ignored\n", 0o644)

	first, _, files, err := ExecutionDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{"mise-tasks/install/docker", "mise.toml", "scripts/bin/hello"}
	if !reflect.DeepEqual(files, wantFiles) {
		t.Fatalf("files = %#v, want %#v", files, wantFiles)
	}

	writeTrustFixture(t, filepath.Join(root, "home", ".zshrc"), "still ignored\n", 0o644)
	ignored, _, _, err := ExecutionDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if ignored != first {
		t.Fatal("managed dotfile unexpectedly changed the executable digest")
	}

	writeTrustFixture(t, filepath.Join(root, "scripts", "bin", "hello"), "#!/bin/sh\nprintf hi\n", 0o755)
	changed, _, _, err := ExecutionDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("personal command content did not change the executable digest")
	}

	if err := os.Chmod(filepath.Join(root, "scripts", "bin", "hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	modeChanged, _, _, err := ExecutionDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if modeChanged == changed {
		t.Fatal("personal command mode did not change the executable digest")
	}
}

func TestTrustStoreRevokesApprovalWhenTaskChanges(t *testing.T) {
	root := t.TempDir()
	writeTrustFixture(t, filepath.Join(root, "mise.toml"), "[tools]\n", 0o644)
	task := filepath.Join(root, "mise-tasks", "install", "sample")
	writeTrustFixture(t, task, "#!/bin/sh\n", 0o755)
	store := TrustStore{Path: filepath.Join(t.TempDir(), "state-trust.toml")}

	trusted, err := store.IsTrusted(root)
	if err != nil || trusted {
		t.Fatalf("initial IsTrusted = %v, %v", trusted, err)
	}
	files, err := store.Trust(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("trusted files = %#v", files)
	}
	trusted, err = store.IsTrusted(root)
	if err != nil || !trusted {
		t.Fatalf("trusted IsTrusted = %v, %v", trusted, err)
	}

	writeTrustFixture(t, task, "#!/bin/sh\nprintf changed\n", 0o755)
	trusted, err = store.IsTrusted(root)
	if err != nil || trusted {
		t.Fatalf("changed IsTrusted = %v, %v", trusted, err)
	}
}

func TestExecutionDigestRejectsSymlinkedCommands(t *testing.T) {
	root := t.TempDir()
	writeTrustFixture(t, filepath.Join(root, "mise.toml"), "[tools]\n", 0o644)
	target := filepath.Join(root, "outside")
	writeTrustFixture(t, target, "#!/bin/sh\n", 0o755)
	commandDir := filepath.Join(root, "scripts", "bin")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(commandDir, "unsafe")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ExecutionDigest(root); err == nil {
		t.Fatal("ExecutionDigest accepted a symlinked command")
	}
}

func TestTaskNameUsesMiseFileTaskGrouping(t *testing.T) {
	cases := map[string]string{
		"mise-tasks/install/docker":         "install:docker",
		".mise/tasks/project/check":         "project:check",
		".config/mise/tasks/setup/_default": "setup",
	}
	for path, want := range cases {
		got, ok := TaskName(path)
		if !ok || got != want {
			t.Errorf("TaskName(%q) = %q, %v; want %q, true", path, got, ok, want)
		}
	}
	if _, ok := TaskName("scripts/bin/hello"); ok {
		t.Fatal("personal command was classified as a mise task")
	}
}

func writeTrustFixture(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
