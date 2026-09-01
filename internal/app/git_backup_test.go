package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderGitBackupGuidesFirstCommitAndPrivateRemote(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "personal state")
	got := renderGitBackup(gitBackupStatus{branch: "main"}, stateDir)
	for _, fragment := range []string{
		"Backup Git", "Repositório", "não iniciado", "Primeiro commit", "pendente",
		"Remoto", "ausente", "git -C '", "init --initial-branch=main",
		"diff --cached", "commit -m 'configura meu ambiente'",
		"gh repo create 'personal state' --private", "URL_DO_REPOSITORIO",
		"nenhum desses comandos foi executado",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("Git backup guidance is missing %q:\n%s", fragment, got)
		}
	}
}

func TestRenderGitBackupKeepsCleanRemoteRepositoryCompact(t *testing.T) {
	got := renderGitBackup(gitBackupStatus{
		repository: true, hasCommit: true, branch: "trunk", remotes: []string{"origin"},
	}, "/tmp/state")
	for _, fragment := range []string{"Primeiro commit", "criado", "Mudanças locais", "nenhuma", "Remoto", "origin"} {
		if !strings.Contains(got, fragment) {
			t.Errorf("clean backup status is missing %q:\n%s", fragment, got)
		}
	}
	for _, unwanted := range []string{"git -C", "gh repo create", "nenhum desses comandos"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("clean backup status contains unnecessary guidance %q:\n%s", unwanted, got)
		}
	}
}

func TestInspectGitBackupUsesOnlyReadOnlyGitQueries(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(stateDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{
		paths: map[string]string{"git": "/bin/git"},
		outputHook: func(call runCall) (string, error) {
			switch strings.Join(call.args, " ") {
			case "rev-parse --is-inside-work-tree":
				return "true\n", nil
			case "branch --show-current":
				return "main\n", nil
			case "rev-parse --verify --quiet HEAD":
				return "", errors.New("unborn branch")
			case "-c core.fsmonitor=false -c core.untrackedCache=false status --porcelain=v1 --untracked-files=all":
				return "?? mise.toml\n", nil
			case "remote":
				return "", nil
			default:
				return "", errors.New("unexpected Git query")
			}
		},
	}
	application := New(Options{Runner: runner, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	got := application.inspectGitBackup(context.Background(), stateDir)
	if !got.repository || got.hasCommit || !got.dirty || got.branch != "main" || len(got.remotes) != 0 || got.inspectError != nil {
		t.Fatalf("Git backup status = %#v", got)
	}
	foundSafeStatus := false
	for _, call := range runner.runs {
		joined := " " + strings.Join(call.args, " ") + " "
		if strings.Contains(joined, " status --porcelain=v1 ") {
			foundSafeStatus = strings.Contains(strings.Join(call.environment, " "), "GIT_OPTIONAL_LOCKS=0") &&
				strings.Contains(joined, " -c core.fsmonitor=false ")
		}
		for _, mutating := range []string{" add ", " commit ", " push ", " init ", " remote add "} {
			if strings.Contains(joined, mutating) {
				t.Fatalf("Git inspection used mutating command %q in %#v", mutating, call)
			}
		}
	}
	if !foundSafeStatus {
		t.Fatalf("Git status was not isolated from optional locks and fsmonitor: %#v", runner.runs)
	}
}

func TestInspectGitBackupRefusesLinkedGitMetadata(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "outside")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(stateDir, ".git")); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]string{"git": "/bin/git"}}
	application := New(Options{Runner: runner})

	got := application.inspectGitBackup(context.Background(), stateDir)
	if !got.repository || got.inspectError == nil || !strings.Contains(got.inspectError.Error(), "diretório real") {
		t.Fatalf("linked Git metadata status = %#v", got)
	}
	if len(runner.runs) != 0 {
		t.Fatalf("linked Git metadata was queried: %#v", runner.runs)
	}
}
