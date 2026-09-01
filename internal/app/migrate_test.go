package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqem/konen/internal/project"
)

func TestMigrateDryRunShowsAllFormatsAndChangesNothing(t *testing.T) {
	fixture := newLegacyMigrationFixture(t)
	beforeConfig := readTestFile(t, fixture.configPath)
	beforeProject := readTestFile(t, fixture.projectPath)
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: fixture.configPath, HomeDir: fixture.home,
		Out: &out, Err: &out, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"migrate", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"Configuração local", "projects/sample.toml", "v0", "v1", "v2", "migrar v0 → v2",
		"--- ", "+version = 1", "+version = 2", "Nenhum arquivo foi alterado",
	} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("migration dry-run is missing %q:\n%s", fragment, out.String())
		}
	}
	if got := readTestFile(t, fixture.configPath); !bytes.Equal(got, beforeConfig) {
		t.Fatalf("dry-run changed config:\n%s", got)
	}
	if got := readTestFile(t, fixture.projectPath); !bytes.Equal(got, beforeProject) {
		t.Fatalf("dry-run changed project:\n%s", got)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(fixture.configPath), "migration-backups", "*")); err != nil || len(matches) != 0 {
		t.Fatalf("dry-run backups = %v, %v", matches, err)
	}
}

func TestMigrateAppliesAtomicallyBacksUpAndInvalidatesProjectTrust(t *testing.T) {
	fixture := newLegacyMigrationFixture(t)
	beforeConfig := readTestFile(t, fixture.configPath)
	beforeProject := readTestFile(t, fixture.projectPath)
	trust := project.TrustStore{Path: filepath.Join(filepath.Dir(fixture.configPath), "projects-trust.toml")}
	if err := trust.Trust(fixture.projectPath); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	runner := &fakeRunner{}
	application := New(Options{
		ConfigPath: fixture.configPath, HomeDir: fixture.home,
		Out: &out, Err: &out, Runner: runner, Prompter: unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"migrate", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if got := string(readTestFile(t, fixture.configPath)); !strings.HasPrefix(got, "version = 1\n") {
		t.Fatalf("migrated config = %q", got)
	}
	if got := string(readTestFile(t, fixture.projectPath)); !strings.HasPrefix(got, "version = 2\n") {
		t.Fatalf("migrated project = %q", got)
	}
	backupRoots, err := filepath.Glob(filepath.Join(filepath.Dir(fixture.configPath), "migration-backups", "*"))
	if err != nil || len(backupRoots) != 1 {
		t.Fatalf("backup roots = %v, %v", backupRoots, err)
	}
	if got := readTestFile(t, filepath.Join(backupRoots[0], "config.toml")); !bytes.Equal(got, beforeConfig) {
		t.Fatalf("config backup = %q", got)
	}
	if got := readTestFile(t, filepath.Join(backupRoots[0], "projects", "sample.toml")); !bytes.Equal(got, beforeProject) {
		t.Fatalf("project backup = %q", got)
	}
	trusted, err := trust.IsTrusted(fixture.projectPath)
	if err != nil || trusted {
		t.Fatalf("migrated project trust = %v, %v", trusted, err)
	}
	for _, fragment := range []string{"Migração concluída: 2 arquivo(s)", "Backup local:", "konen project trust sample"} {
		if !strings.Contains(out.String(), fragment) {
			t.Errorf("migration output is missing %q:\n%s", fragment, out.String())
		}
	}
	if len(runner.runs) != 0 {
		t.Fatalf("migration executed external commands: %#v", runner.runs)
	}
}

func TestMigrateReportsCurrentFormatsWithoutCreatingBackup(t *testing.T) {
	fixture := newLegacyMigrationFixture(t)
	configLegacy := readTestFile(t, fixture.configPath)
	if err := os.WriteFile(fixture.configPath, append([]byte("version = 1\n"), configLegacy...), 0o600); err != nil {
		t.Fatal(err)
	}
	projectLegacy := readTestFile(t, fixture.projectPath)
	if err := os.WriteFile(fixture.projectPath, append([]byte("version = 2\n"), projectLegacy...), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	application := New(Options{
		ConfigPath: fixture.configPath, HomeDir: fixture.home,
		Out: &out, Err: &out, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
	})

	if err := application.Run(context.Background(), []string{"migrate"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "já compatível") || !strings.Contains(out.String(), "Nenhuma migração necessária") {
		t.Fatalf("current migration output = %s", out.String())
	}
}

func TestMigrateRefusesFutureFormatWithoutWriting(t *testing.T) {
	fixture := newLegacyMigrationFixture(t)
	future := []byte(fmt.Sprintf("version = 99\nstate_dir = %q\n", fixture.stateDir))
	if err := os.WriteFile(fixture.configPath, future, 0o600); err != nil {
		t.Fatal(err)
	}
	application := New(Options{
		ConfigPath: fixture.configPath, HomeDir: fixture.home,
		Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
	})

	err := application.Run(context.Background(), []string{"migrate", "--yes"})
	if err == nil || !strings.Contains(err.Error(), "konen update") {
		t.Fatalf("future migration error = %v", err)
	}
	if got := readTestFile(t, fixture.configPath); !bytes.Equal(got, future) {
		t.Fatalf("future config changed: %q", got)
	}
}

func TestMigrateRefusesLinksAndFilesChangedDuringReview(t *testing.T) {
	t.Run("symbolic config", func(t *testing.T) {
		fixture := newLegacyMigrationFixture(t)
		realConfig := fixture.configPath + ".real"
		if err := os.Rename(fixture.configPath, realConfig); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realConfig, fixture.configPath); err != nil {
			t.Fatal(err)
		}
		application := New(Options{
			ConfigPath: fixture.configPath, HomeDir: fixture.home,
			Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: &fakeRunner{}, Prompter: unusedPrompter{},
		})
		err := application.Run(context.Background(), []string{"migrate", "--dry-run"})
		if err == nil || !strings.Contains(err.Error(), "apenas arquivos regulares") {
			t.Fatalf("symbolic migration error = %v", err)
		}
	})

	t.Run("changed after confirmation", func(t *testing.T) {
		fixture := newLegacyMigrationFixture(t)
		prompter := mutateConfirmPrompter{mutate: func() {
			file, err := os.OpenFile(fixture.configPath, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("# mudou durante a revisão\n"); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}}
		application := New(Options{
			ConfigPath: fixture.configPath, HomeDir: fixture.home, Interactive: true,
			Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Runner: &fakeRunner{}, Prompter: prompter,
		})
		err := application.Run(context.Background(), []string{"migrate"})
		if err == nil || !strings.Contains(err.Error(), "mudou durante a revisão") {
			t.Fatalf("concurrent migration error = %v", err)
		}
		if strings.HasPrefix(string(readTestFile(t, fixture.configPath)), "version = 1\n") {
			t.Fatal("changed config was migrated")
		}
	})
}

type mutateConfirmPrompter struct {
	unusedPrompter
	mutate func()
}

func (p mutateConfirmPrompter) Confirm(string) (bool, error) {
	p.mutate()
	return true, nil
}

type legacyMigrationFixture struct {
	home        string
	stateDir    string
	configPath  string
	projectPath string
}

func newLegacyMigrationFixture(t *testing.T) legacyMigrationFixture {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	stateDir := filepath.Join(home, "state")
	configPath := filepath.Join(home, ".config", "konen", "config.toml")
	projectPath := filepath.Join(stateDir, "projects", "sample.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "mise.toml"), []byte("[tools]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("state_dir = %q\n", stateDir)), 0o600); err != nil {
		t.Fatal(err)
	}
	projectData := []byte("path = '~/project'\n\n[[tabs]]\ntitle = 'Terminal'\n")
	if err := os.WriteFile(projectPath, projectData, 0o644); err != nil {
		t.Fatal(err)
	}
	return legacyMigrationFixture{home: home, stateDir: stateDir, configPath: configPath, projectPath: projectPath}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
