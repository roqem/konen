# Konen

> Do zero à sua máquina.

Konen is a [Roqem](https://github.com/roqem) project. Its name comes from the
Hebrew כּוֹנֵן (*konen*): to establish or build.

Konen is a small, friendly front end for rebuilding a workstation from plain,
versionable state. It provides the guided experience; [mise](https://mise.jdx.dev/)
does the package, tool, repository and dotfile convergence.

The current state contract requires mise 2026.8.15 or newer.

Konen is intentionally not another package manager and does not invent a
private convergence format for your machine. Its small project manifests only
describe how a workspace is opened; mise remains the source of truth for the
machine itself.

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
konen init --git ~/home
konen trust
konen add ~/.zshrc
konen status
konen diff
konen apply --dry-run
konen apply
konen doctor
```

Projects and their terminal tabs live in the same central state, rather than
being scattered as Kitty files across source repositories:

```console
cd ~/Documents/Projects/my-app
konen project add
konen dev --dry-run
konen dev
```

`konen dev` infers the project from the current directory. A name is useful
from anywhere (`konen dev my-app`). Inside Kitty it opens tabs in the current OS
window; outside Kitty it starts a native Kitty session in a new window. Project
commands must be approved locally and any out-of-band manifest change revokes
that approval. See [docs/projects.md](docs/projects.md).

An existing state repository can be cloned directly:

```console
konen init --from git@github.com:you/home.git
less ~/.local/share/konen/state/mise.toml
konen trust
```

Fresh state created by Konen is trusted automatically. Existing folders and
cloned repositories require an explicit `konen trust` after inspection.

## State

The directory and repository name are the user's choice; `home` is only an
example. The initial state is deliberately ordinary:

```text
home/
├── .git/          # optional
├── .gitignore
├── mise.toml      # packages, tools, repos, services and dotfiles
├── home/          # source files managed by mise
└── projects/      # Konen workspace manifests
```

Konen stores only a pointer to that directory in
`~/.config/konen/config.toml`. The generated state also links its `mise.toml`
as mise's global user config, so machine-wide tools remain available inside
projects and project-local mise files can override them. Konen never commits or
pushes on your behalf.

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
