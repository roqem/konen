# Testing and release qualification

## Automated gate

Run the complete local gate with:

```console
mise run check
```

Besides formatting, static analysis and unit tests, `go test ./...` includes two
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
version. Replace `v0.1.0-alpha.5` below if the candidate has another version.
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
KONEN_VERSION=v0.1.0-alpha.5 sh install.sh
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
konen plan
konen apply --dry-run
konen apply
konen status
git -C ~/.local/share/konen/state status --short
```

Expected results:

- `doctor` recognizes the co-installed mise and the state directory;
- the first dry run explains the temporary mise warning about the global
  config that does not exist until apply, and describes changes without
  applying them;
- the real apply converges successfully inside the disposable VM;
- the final status has no unexpected pending resources;
- the state is an ordinary Git repository, Konen explains that it did not
  commit anything, and its initial files are visible as untracked;

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
