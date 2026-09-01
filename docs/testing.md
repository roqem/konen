# Testing and release qualification

## Automated gate

Run the complete local gate with:

```console
mise run check
```

Besides formatting, static analysis and unit tests, `go test ./...` includes
Linux integration journeys:

- the installer downloads real archives from an isolated local HTTP server,
  installs Konen and mise into a temporary home, upgrades Konen without losing
  state, rejects a corrupted archive and proves that `sudo` was not invoked;
- a built Konen executable runs `init --git`, `doctor`, `status`, `plan`,
  `apply --dry-run`, completion generation and the project inspection/trust
  workflow with temporary configuration, state, home and backend binaries;
- unit journeys prove that public GitHub state does not trigger authentication,
  while a failed private HTTPS clone uses device login and a repository-scoped
  helper without creating SSH keys or changing global Git configuration.

The integration harness never applies the developer's machine state and never
uses the developer's home directory.

## Manual test on a clean Linux VM

The final release qualification must use a disposable VM and a published test
version. Replace `v0.1.0-alpha.12` below if the candidate has another version.
Tags with a prerelease suffix are published as GitHub prereleases and are not
selected by an unpinned installer invocation.

Install only the bootstrap prerequisites using the VM's package manager. On
Ubuntu or Debian:

```console
sudo apt update
sudo apt install --yes ca-certificates curl git
```

Download and inspect the installer, then ask it for the exact candidate:

```console
curl -fsSLO https://raw.githubusercontent.com/roqem/konen/main/install.sh
less install.sh
KONEN_VERSION=v0.1.0-alpha.12 sh install.sh
export PATH="$HOME/.local/bin:$PATH"
konen version
```

The installation must finish without asking for root, install `konen` and
`mise` under `~/.local/bin`, and print the candidate version.

Exercise the interactive first run:

```console
konen
```

Choose **Configurar o Konen**, accept the default state directory and allow it
to initialize Git. Then run:

```console
konen doctor
konen status
konen plan --select
konen tool add --dry-run node lts
konen tool add --yes node lts
konen package add --dry-run --manager apt jq latest
konen package add --yes --manager apt jq latest
konen repo add --dry-run ~/src/konen-docs-test https://github.com/roqem/konen.git main
konen command add --dry-run work-note
konen command add --yes work-note
konen status
konen plan
konen plan --only packages
konen plan --only tools,dotfiles
konen apply --dry-run
konen apply
konen status
git -C ~/.local/share/konen/state status --short
```

Expected results:

- `doctor` recognizes the co-installed mise and the state directory;
- `status` reports the pending first Git commit and missing remote, shows the
  manual review/commit/private-remote commands and explicitly says it ran none
  of them;
- the initial selector displays its single `Dotfiles` option instead of an
  empty list;
- the guided tool dry run displays the exact `mise.toml` diff without writing
  it, while the confirmed command adds `node = "lts"` and refreshes trust;
- the package assistant explains apt and its platform, previews the native
  declaration, and the confirmed command edits state without installing `jq`;
- the repository dry run previews its destination, URL and ref without cloning
  anything;
- the command dry run displays the complete safe scaffold, while the confirmed
  command creates an executable `scripts/bin/work-note`, does not run it and
  makes it visible as `Comando pessoal` in status;
- dry runs use only the selected state, without merging a previously linked
  global config or ancestor version files;
- `--only tools,dotfiles` limits the plan to those bootstrap phases;
- the real apply converges successfully inside the disposable VM;
- the final status has no unexpected pending resources;
- the state is an ordinary Git repository, Konen explains that it did not
  commit anything, and its initial files are visible as untracked.

After the real apply, exercise installer authoring without adding an incomplete
task to the applied journey:

```console
konen installer add --dry-run browser-test
printf '%s\n' \
  '#!/bin/sh' \
  '#MISE description="Harmless installer test"' \
  'set -eu' \
  'exit 0' \
  > "$HOME/install-noop"
konen installer add --dry-run --from "$HOME/install-noop" noop
konen installer add --yes --from "$HOME/install-noop" noop
cmp "$HOME/install-noop" \
  "$HOME/.local/share/konen/state/mise-tasks/install/noop"
konen status
konen plan --only task
```

The scaffold preview must show an executable that exits unsuccessfully and a
sequential `install:browser-test` selection, while writing neither file. The
import must copy exact bytes with mode `0755`, append only `install:noop` to the
native bootstrap list, refresh trust and never execute it. Status must list it
as `Instalador pessoal`; the task-only plan may print the command but must not
run it.

Check completion in the VM's shell. For Bash:

```console
eval "$(konen completion bash)"
```

Typing `konen p` followed by Tab should offer `plan`, `project` and `projects`.

Exercise private remote bootstrap in an isolated Konen configuration, replacing
the repository with one the tester can read:

```console
export XDG_CONFIG_HOME="$HOME/.config-konen-remote-test"
konen init --from github:OWNER/REPOSITORY "$HOME/remote-state-test"
konen status
konen trust
konen status
konen plan
```

The initial clone must fail without a Git username prompt, continue through
GitHub CLI device login and then succeed over HTTPS. Before `konen trust`,
`status` must return a Konen error without displaying mise's trust prompt. After
trust, `status` must distinguish requested, resolved and installed tool state,
and `plan` must show the full bootstrap dry run rather than `nothing configured`.
Compare `git config --global --show-origin --get-all
credential.https://github.com.helper` before and after the clone: it must not
change. If the initial clone failed and the assisted fallback ran, the same key
queried with `git -C "$HOME/remote-state-test" config --local --get-all` must
show an empty reset entry followed by the machine-local GitHub CLI helper. If
pre-existing authentication made the initial clone succeed, no local entry is
added and the existing mechanism remains untouched.

If the remote state contains personal commands or custom installers, verify
the content-aware approval boundary before applying it:

```console
konen status
konen plan
printf '\n# trust probe\n' >> "$HOME/remote-state-test/mise-tasks/install/EXAMPLE"
konen status
git -C "$HOME/remote-state-test" restore mise-tasks/install/EXAMPLE
konen trust
konen status
```

Replace `EXAMPLE` with a real installer task from the state. The first status
must list it as an `Instalador pessoal`, and the plan must print its file-task
command without executing it. After the edit, status must stop before invoking
mise and require `konen trust`. Restoring the exact approved bytes and file mode
restores the matching approval; keeping the edit requires a new explicit trust.
Also add a temporary executable under `scripts/bin`, repeat the check and
confirm it is listed as `Comando pessoal`. Symlinks in either executable
directory must be rejected rather than approved.

Finally, exercise a project without requiring a graphical terminal:

```console
mkdir -p ~/Projects/example
git -C ~/Projects/example init --initial-branch=main
konen project add ~/Projects/example
konen projects
konen example --dry-run
```

Accept the default shell-only `Terminal` tab. The project must appear as
approved, and the dry run must show the exact directory, tab title and empty
command. Opening the tabs for real is a separate optional check on a graphical
VM with Kitty and `allow_remote_control yes`.

Finally, edit the manifest outside Konen and prove that execution is blocked
until the new digest is reviewed and approved:

```console
sed -i "s/title = 'Terminal'/title = 'Terminal externo'/" \
  ~/.local/share/konen/state/projects/example.toml
konen example
konen project show example
konen project trust example
konen example --dry-run
```

Record the VM distribution, architecture, candidate version and any command
whose result differs from this checklist. Revert the VM snapshot afterward;
the test is deliberately allowed to change that disposable home.

## Latest qualification record

`v0.1.0-alpha.6` passed the complete manual journey on 2026-08-31 in a clean
Multipass VM running Ubuntu 26.04 LTS on `linux/amd64` with two CPUs and 4 GiB
of memory. The private representative state required GitHub device login and
then converged:

- 24 apt packages and three auxiliary Git repositories;
- eight dotfile entries;
- 17 versioned user-space tools;
- Chrome 152.0.7977.64, Kitty 0.45.0 and Neovim 0.12.5;
- Docker Engine 29.7.2 with non-root access after a new login session;
- Zsh as the account login shell, including a passwordless Multipass account.

A second `konen plan` reported every declarative resource current. A second
`konen apply --yes` invoked the four selected personal tasks, whose idempotence
guards exited without reinstalling or changing the machine. `konen doctor`
reported configuration, state, executable-surface approval, mise and Git as
healthy.
