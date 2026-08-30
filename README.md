# Zeroot

> Do zero à sua máquina.

Zeroot is a small, friendly front end for rebuilding a workstation from plain,
versionable state. It provides the guided experience; [mise](https://mise.jdx.dev/)
does the package, tool, repository and dotfile convergence.

The current state contract requires mise 2026.8.14 or newer.

Zeroot is intentionally not another package manager and does not invent a
private configuration format for your machine.

## Status

Pre-alpha. The state format and the first local workflow are implemented, but
there is no published release or one-line installer yet.

## Commands

Running `zeroot` opens an interactive menu. The same operations remain available
as small commands for scripts and recovery environments:

```console
zeroot init ~/my-machine --git
zeroot add ~/.zshrc
zeroot status
zeroot diff
zeroot apply --dry-run
zeroot apply
zeroot doctor
```

An existing state repository can be cloned directly:

```console
zeroot init --from git@github.com:you/my-machine.git
```

## State

The initial state is deliberately ordinary:

```text
my-machine/
├── .git/          # optional
├── .gitignore
├── mise.toml      # packages, tools, repos, services and dotfiles
└── home/          # source files managed by mise
```

Zeroot stores only a pointer to that directory in
`~/.config/zeroot/config.toml`. It never commits or pushes on your behalf.

## Development

The repository dogfoods mise:

```console
mise install
mise run check
```

Without mise, use Go 1.25.8 or newer:

```console
go test ./...
go build ./cmd/zeroot
```

See [docs/architecture.md](docs/architecture.md) for the scope and extension
decisions.

## License

Apache-2.0.
