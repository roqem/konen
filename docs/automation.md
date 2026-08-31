# Personal commands and custom installers

Konen keeps custom automation in the user's state repository without defining
a second task language. Mise remains the task runner and bootstrap engine;
Konen makes the executable surface visible and requires content-aware local
approval.

## Personal commands

Executable files under `scripts/bin` are added to `PATH` when mise is active.
The generated `mise.toml` resolves this directory from the real state checkout,
even though that file is later linked as mise's global configuration.

```text
scripts/bin/
└── work-note
```

```sh
#!/bin/sh
set -eu
printf '%s\n' "${1:-Remember to document this command.}"
```

Prefer a small command over a shell alias when it contains logic: commands work
across shells, can be tested directly and can be used in project tab commands.
Simple interactive abbreviations may still remain shell aliases.

Create a safe scaffold or import an existing command through the guided flow:

```console
konen command add work-note
konen command add --from ~/bin/work-note
```

Both forms display the complete proposed executable and ask before writing.
`--dry-run` stops after that preview; `--yes` confirms only the file creation.
Imported commands must be regular UTF-8 text files with a shebang and are
copied rather than linked. Konen writes mode `0755`, refreshes local approval
and does not execute the command.

## Custom installers

An installer is an executable mise file task grouped under `install`:

```text
mise-tasks/install/example
```

```sh
#!/bin/sh
#MISE description="Instala o aplicativo Example"
set -eu

command -v example >/dev/null 2>&1 && exit 0
# Perform the smallest idempotent installation that remains necessary.
```

Konen can create a deliberately incomplete scaffold or import an existing
installer, then select its native mise task in the sequential bootstrap list:

```console
konen installer add example
konen installer add --from ~/bin/install-example
```

Both forms display the complete executable and the `mise.toml` diff before
writing. `--dry-run` stops after the preview; `--yes` confirms the two writes,
not task execution. Imported files must be regular UTF-8 text files with a
shebang and are copied with mode `0755`. The generated scaffold contains no
installation command and exits unsuccessfully until implemented, preventing a
future apply from reporting a no-op installer as successful. After editing it,
run `konen trust` before planning or applying the state.

It is available explicitly as `mise run install:example`. The guided flow
creates this native selection in `mise.toml`; a manually created installer can
declare the same dependency directly:

```toml
[tasks.bootstrap]
run = [
  { task = "install:example" },
]
```

Use sequential task references for installers that invoke a system package
manager. Mise runs ordinary task dependencies in parallel, which would make
multiple APT installers compete for the same lock.

`konen status` identifies the task and its source file. `konen plan` delegates
to `mise bootstrap --dry-run`, which prints the selected file-task command
without running it. `konen apply` runs the task after declarative resources and
tools; installers must therefore be idempotent.

Use declarative state whenever it fits:

- `[bootstrap.packages]` for apt, dnf, pacman, brew, flatpak and other host
  packages;
- `[tools]` for versioned user-space development tools;
- `[bootstrap.repos]` for Git checkouts;
- `[dotfiles]` for managed configuration;
- `mise-tasks/install` only for setup that those sections cannot express.

Do not download a remote script and pipe it directly into a shell. Prefer the
vendor's signed package repository or release artifact, validate downloaded
metadata where practical, and make every privileged command visible in the
versioned task.

Mise enables tool auto-installation for `mise run` and `mise exec` by default.
Once this state is the active global config, invoking mise directly may install
missing entries from `[tools]` even without `konen apply`; this is mise behavior,
not a hidden Konen phase. `konen status` and `konen plan` remain read-only. Users
who prefer strictly explicit tool installation can change mise's
`auto_install` setting, accepting that project tasks will then require a prior
`mise install` or bootstrap.

## Approval boundary

Konen's local approval digest contains:

- `mise.toml`;
- all standard mise file-task directories inside the state;
- `scripts/bin`;
- file permission bits, including whether a file became executable.

Changing any of these files requires another `konen trust` before `status`,
`plan`, `diff`, dotfile capture or `apply` can invoke mise. Executable-surface
symlinks are refused. Managed dotfiles and project manifests retain their own
normal workflows: project commands continue to use per-project approval.
