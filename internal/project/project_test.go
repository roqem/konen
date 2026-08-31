package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTripAndDirectoryMatch(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	work := filepath.Join(home, "Documents", "Projects", "sample")
	if err := os.MkdirAll(filepath.Join(work, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := Store{StateDir: filepath.Join(root, "state"), HomeDir: home}
	manifest := Manifest{
		Version: manifestVersion,
		Path:    "~/Documents/Projects/sample",
		Tabs: []Tab{
			{Title: "Editor", Command: "nvim ."},
			{Title: "Terminal"},
		},
	}
	path, err := store.Save("sample", manifest)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "state", "projects", "sample.toml") {
		t.Fatalf("path = %q", path)
	}
	got, _, err := store.Load("sample")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != manifest.Path || len(got.Tabs) != 2 || got.Tabs[0].Command != "nvim ." {
		t.Fatalf("manifest = %#v", got)
	}
	matches, err := store.MatchDirectory(filepath.Join(work, "src"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Name != "sample" {
		t.Fatalf("matches = %#v", matches)
	}
}

func TestTrustChangesWithManifest(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "state", "projects", "sample.toml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	trust := TrustStore{Path: filepath.Join(root, "config", "projects-trust.toml")}
	if trusted, err := trust.IsTrusted(manifestPath); err != nil || trusted {
		t.Fatalf("IsTrusted() = %v, %v", trusted, err)
	}
	if err := trust.Trust(manifestPath); err != nil {
		t.Fatal(err)
	}
	if trusted, err := trust.IsTrusted(manifestPath); err != nil || !trusted {
		t.Fatalf("IsTrusted() = %v, %v", trusted, err)
	}
	if err := os.WriteFile(manifestPath, []byte("version = 1\npath = 'changed'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if trusted, err := trust.IsTrusted(manifestPath); err != nil || trusted {
		t.Fatalf("changed IsTrusted() = %v, %v", trusted, err)
	}
}

func TestPortablePath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	got, err := PortablePath(filepath.Join(home, "Documents", "Project"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got != "~/Documents/Project" {
		t.Fatalf("PortablePath() = %q", got)
	}
}

func TestKeepInvokingTabDefaultsToTrue(t *testing.T) {
	manifest := Manifest{}
	if !manifest.KeepsInvokingTab() {
		t.Fatal("missing keep_invoking_tab should preserve the caller")
	}
	keep := false
	manifest.KeepInvokingTab = &keep
	if manifest.KeepsInvokingTab() {
		t.Fatal("explicit false should close the caller")
	}
}
