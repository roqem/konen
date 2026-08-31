package execx

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type Runner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, dir, name string, args ...string) error
	RunEnv(ctx context.Context, dir string, environment []string, name string, args ...string) error
	Output(ctx context.Context, dir, name string, args ...string) (string, error)
}

type OSRunner struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

func (r OSRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (r OSRunner) Run(ctx context.Context, dir, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdin = r.In
	command.Stdout = r.Out
	command.Stderr = r.Err
	return command.Run()
}

func (r OSRunner) RunEnv(ctx context.Context, dir string, environment []string, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), environment...)
	command.Stdin = r.In
	command.Stdout = r.Out
	command.Stderr = r.Err
	return command.Run()
}

func (r OSRunner) Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdin = r.In
	command.Stderr = r.Err
	output, err := command.Output()
	return string(output), err
}
