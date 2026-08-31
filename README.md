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
implemented, and the first candidate is published as a prerelease for clean-VM
qualification. There is no stable release yet.

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

### Shell completion

Konen generates completion for every command, option, path and dynamic project
name. Add the line for your shell to its startup file:

```zsh
eval "$(konen completion zsh)"
```

```bash
eval "$(konen completion bash)"
```

Fish can load the generated script from its standard completion directory:

```fish
konen completion fish > ~/.config/fish/completions/konen.fish
```

## Commands

Running `konen` opens an interactive menu. The same operations remain available
as small commands for scripts and recovery environments:

```console
konen init --git ~/home
konen trust
konen dotfile add ~/.zshrc
konen status
konen plan
konen diff
konen apply --dry-run
konen apply
konen doctor
```

Menu entries show the corresponding command. Press `q`, Escape or Ctrl+C to
leave without treating cancellation as an error.

Projects and their terminal tabs live in the same central state, rather than
being scattered as Kitty files across source repositories:

```console
cd ~/Documents/Projects/my-app
konen project add
konen projects
konen dev --dry-run
konen dev
```

`konen dev` infers the project from the current directory. A name is useful
from anywhere (`konen dev my-app`); a registered project can also be opened by
the short form `konen my-app`. Inside Kitty it opens tabs in the current OS
window; outside Kitty it starts a native Kitty session in a new window. Project
commands must be approved locally and any out-of-band manifest change revokes
that approval. See [docs/projects.md](docs/projects.md).

An existing state repository can be cloned directly. The `github:` form uses
HTTPS and, if a private clone requires authentication, guides a device-code
login that may be completed in a browser on another device:

```console
konen init --from github:you/home
less ~/.local/share/konen/state/mise.toml
konen trust
```

SSH remains optional. When GitHub CLI is unavailable, Konen transparently uses
the co-installed mise to run `gh@latest` for authentication without sudo; this
bootstrap helper is not added to the user's machine state. If GitHub CLI has
multiple accounts, Konen verifies repository access and explicitly asks it to
switch accounts when the active one cannot read the selected state.

When an anonymous clone fails and the assisted path is needed, its retry
receives the GitHub CLI credential helper only for that command and records it
only in the cloned repository's local `.git/config`. A clone already satisfied
by existing Git authentication is left as-is. Konen does not run
`gh auth setup-git` or alter either global Git config file.
For portable preferences, manage `~/.config/git/config`; `konen dotfile add`
refuses whole `~/.gitconfig` captures and known plaintext credential files.

Fresh state created by Konen is trusted automatically. Existing folders and
cloned repositories require an explicit `konen trust` after inspection.
Until then, Konen refuses every mise-backed operation — including `status`,
`plan`, `diff`, dotfile capture and `apply` — instead of allowing mise to open
its own trust prompt.

## State

The directory and repository name are the user's choice; `home` is only an
example. The initial state is deliberately ordinary:

```text
home/
├── .git/          # optional
├── .gitignore
├── mise.toml      # packages, tools, repos, services and dotfiles
├── mise-tasks/
│   └── install/   # explicit, idempotent custom installers
├── scripts/
│   └── bin/       # personal commands added to PATH by mise activation
├── home/          # source files managed by mise
└── projects/      # Konen workspace manifests
```

Konen stores only a pointer to that directory in
`~/.config/konen/config.toml`. The generated state also links its `mise.toml`
as mise's global user config, so machine-wide tools remain available inside
projects and project-local mise files can override them. Konen never commits or
pushes on your behalf.

After trust, `konen status` lists every package, tool, service, repository and
managed file declared by the state, plus personal commands and file-based
installer tasks. `konen plan` runs the complete bootstrap dry run, showing
dotfiles, tools and every custom installer selected by the `bootstrap` task
before `konen apply`; Konen does not keep a second, hidden installation list.

Custom behavior remains native mise state. Put directly invoked utilities in
`scripts/bin`, executable installer tasks in `mise-tasks/install`, and select
automatic installers through sequential task references in `[tasks.bootstrap]`.
Konen hashes the contents and executable modes of `mise.toml`, every recognized
mise file-task directory and `scripts/bin`; changing any of them blocks
state-backed commands until `konen trust` is run again. Symbolic links are
refused on this executable surface. See
[docs/automation.md](docs/automation.md).

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
decisions. Release candidates must also pass the isolated installer and
first-run journeys described in [docs/testing.md](docs/testing.md).

## License

Apache-2.0.
