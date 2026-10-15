//go:build linux

package app

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// A dry run or the appearance of a prompt does not prove the replacement shell
// can read its TTY. Exercise real Zsh job control and send a second command after
// the first shell exits. The PTY fixture reproduces Kitty's run-then-exec flow
// without requiring a graphical session or an installed Kitty in CI.
func TestHeldProjectCommandReturnsUsableShell(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for the PTY regression")
	}
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is required for the job-control regression")
	}
	for _, test := range []struct {
		name    string
		command string
		status  int
		output  string
	}{
		{"external", `/bin/echo TASK_DONE`, 0, "TASK_DONE"},
		{"comment", `/bin/echo TASK_DONE # trailing comment`, 0, "TASK_DONE"},
		{"quoting", `/bin/printf '%s\n' "It's literal: \$HOME"`, 0, "It's literal: $HOME"},
		{"function", `probe() { /bin/echo TASK_DONE; }; probe`, 0, "TASK_DONE"},
		{"failure", `/bin/sh -c 'exit 17'`, 17, ""},
		{"signal", `/bin/sh -c 'kill -TERM $$'`, 143, ""},
		{"interrupt", `/bin/sh -c 'printf WAIT_FOR_INTERRUPT; read answer'`, 130, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, python, filepath.Join("testdata", "hold_shell.py"),
				zsh, projectTabShellCommand(test.command, true), t.TempDir(), strconv.Itoa(test.status), test.output)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("PTY shell-return regression: %v\n%s", err, output)
			}
		})
	}
}
