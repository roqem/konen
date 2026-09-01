package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	want := Config{StateDir: "/tmp/estado com espaço"}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Version != currentVersion || got.StateDir != want.StateDir {
		t.Fatalf("Load() = %#v, want state %q and version %d", got, want.StateDir, currentVersion)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestDefaultPathUsesKonenNamespace(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configHome, "konen", "config.toml")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("version = 99\nstate_dir = '/tmp/state'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "konen update") {
		t.Fatalf("Load() error = %v, want update guidance", err)
	}
}

func TestPlanMigrationAddsVersionToLegacyConfigWithoutChangingItsState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	legacy := []byte("# configuração antiga\nstate_dir = '/tmp/state'\n")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanMigration(path)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Needed() || plan.FromVersion != 0 || plan.ToVersion != 1 {
		t.Fatalf("migration = %#v", plan)
	}
	if !strings.HasPrefix(string(plan.After), "version = 1\n") || plan.Config.StateDir != "/tmp/state" {
		t.Fatalf("migrated config = %q, value = %#v", plan.After, plan.Config)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(legacy) {
		t.Fatalf("planning changed the config: %q", got)
	}
}

func TestLoadGuidesLegacyConfigToExplicitMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("state_dir = '/tmp/state'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "konen migrate --dry-run") {
		t.Fatalf("Load() error = %v, want migration guidance", err)
	}
}
