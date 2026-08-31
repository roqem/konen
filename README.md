# Konen

> Do zero à sua máquina.

Konen is a [Roqem](https://github.com/roqem) project. Its name comes from the
Hebrew כּוֹנֵן (*konen*): to establish or build.

Konen is a small, friendly front end for rebuilding a workstation from plain,
versionable state. It provides the guided experience; [mise](https://mise.jdx.dev/)
does the package, tool, repository and dotfile convergence.

The current state contract requires mise 2026.8.15 or newer.

Konen is intentionally not another package manager and does not invent a
private configuration format for your machine.

## Status

Pre-alpha. The state format, local workflow and release infrastructure are
implemented, but there is no published release yet.

## Installation

After the first public release, the complete interactive entry point will be:

```console
curl -fsSL https://raw.githubusercontent.com/roqem/konen/main/install.sh | sh && ~/.local/bin/konen
```

The installer uses no sudo, verifies release checksums and installs a compatible
mise beside Konen when necessary. To inspect the script before executing it:

```console
curl -fsSLO https://raw.githubusercontent.com/roqem/konen/main/install.sh
less install.sh
sh install.sh
~/.local/bin/konen
```

See [docs/distribution.md](docs/distribution.md) for version pinning, mirrors,
artifact attestations and the trust model.

## Commands

Running `konen` opens an interactive menu. The same operations remain available
as small commands for scripts and recovery environments:

```console
konen init --git ~/my-machine
konen trust
konen add ~/.zshrc
konen status
konen diff
konen apply --dry-run
konen apply
konen doctor
```

An existing state repository can be cloned directly:

```console
konen init --from git@github.com:you/my-machine.git
less ~/.local/share/konen/state/mise.toml
konen trust
```

Fresh state created by Konen is trusted automatically. Existing folders and
cloned repositories require an explicit `konen trust` after inspection.

## State

The initial state is deliberately ordinary:

```text
my-machine/
├── .git/          # optional
├── .gitignore
├── mise.toml      # packages, tools, repos, services and dotfiles
└── home/          # source files managed by mise
```

Konen stores only a pointer to that directory in
`~/.config/konen/config.toml`. It never commits or pushes on your behalf.

## Development

The repository dogfoods mise:

```console
mise install
mise run check
```

Without mise, use Go 1.27.0 or newer:

```console
go test ./...
go build ./cmd/konen
```

See [docs/architecture.md](docs/architecture.md) for the scope and extension
decisions.

## License

Apache-2.0.
