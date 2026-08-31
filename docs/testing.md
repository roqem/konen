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
  workflow with temporary configuration, state, home and backend binaries.

The integration harness never applies the developer's machine state and never
uses the developer's home directory.

## Manual test on a clean Linux VM

The final release qualification must use a disposable VM and a published test
version. Replace `v0.1.0-alpha.1` below if the candidate has another version.
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
KONEN_VERSION=v0.1.0-alpha.1 sh install.sh
export PATH="$HOME/.local/bin:$PATH"
konen version
```

The installation must finish without asking for root, install `konen` and
`mise` under `~/.local/bin`, and print the candidate version.

Exercise the interactive first run:

```console
konen
```

Choose **Configurar o Konen**, create `~/home`, and allow it to initialize Git.
Then run:

```console
konen doctor
konen status
konen plan
konen apply --dry-run
konen apply
konen status
git -C ~/home status --short
```

Expected results:

- `doctor` recognizes the co-installed mise and the state directory;
- the dry run describes changes without applying them;
- the real apply converges successfully inside the disposable VM;
- the final status has no unexpected pending resources;
- `~/home` is an ordinary Git repository and Konen did not commit anything.

Check completion in the VM's shell. For Bash:

```console
eval "$(konen completion bash)"
```

Typing `konen p` followed by Tab should offer `plan`, `project` and `projects`.

Finally, exercise a project without requiring a graphical terminal:

```console
mkdir -p ~/Projects/example
git -C ~/Projects/example init --initial-branch=main
cd ~/Projects/example
konen project add
konen projects
konen dev --dry-run
```

Create at least one shell-only tab in the guided form. The project must appear
as approved, and the dry run must show the exact directory, tab title and
command. Opening the tabs for real is a separate optional check on a graphical
VM with Kitty and `allow_remote_control yes`.

Record the VM distribution, architecture, candidate version and any command
whose result differs from this checklist. Revert the VM snapshot afterward;
the test is deliberately allowed to change that disposable home.
