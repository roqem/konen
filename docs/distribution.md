# Distribution

## Release artifacts

A tag named `vX.Y.Z` triggers the release workflow. The workflow runs formatting,
static analysis and tests, then publishes deterministic archives for Linux
`amd64` and `arm64` plus `checksums.txt`. Only after publication, a second job
uses `scripts/smoke-release.sh` to download that public archive through
`install.sh`, install Konen and mise into a disposable home, initialize state,
apply and reapply its dotfile phase and verify that neither sudo, Git commits
nor remotes appeared.

Archives are built by `scripts/build-release.sh`, not by an opaque release
service. They contain only the Konen executable, README and Apache-2.0 license.
GitHub's build-provenance action also attests the archives.

The smoke test deliberately validates the published download rather than a
binary left in the workflow workspace. It can also qualify an existing release
locally without touching the developer's home:

```console
scripts/smoke-release.sh vX.Y.Z
```

## Installer

`install.sh` installs into `~/.local/bin` without sudo. It:

1. resolves the latest Konen release, unless `KONEN_VERSION` is set;
2. detects Linux architecture;
3. downloads the matching archive and verifies its SHA-256 checksum;
4. installs the Konen executable;
5. keeps a compatible mise already on `PATH`, or installs the pinned mise
   release beside Konen after verifying mise's official checksum.

Konen searches for mise beside its own executable first and on `PATH` second.
The co-installed, compatible backend therefore wins over an older system mise,
and the installation works before the user has added `~/.local/bin` to their
shell configuration.

Supported overrides:

- `KONEN_VERSION=vX.Y.Z`
- `KONEN_INSTALL_DIR=/absolute/path`
- `KONEN_INSTALL_MISE=0`
- `KONEN_MISE_VERSION=YYYY.M.PATCH`
- `KONEN_REPOSITORY=owner/repository`

The release-base URL overrides exist for mirrors and integration tests:
`KONEN_RELEASE_BASE_URL` and `KONEN_MISE_RELEASE_BASE_URL`.

## In-product updates

`konen update` uses the same release archives and checksum contract as the
installer. Its read-only plan resolves release metadata through the GitHub API,
shows current and target versions and identifies which manager owns each
executable. Prerelease builds follow the newest published prerelease; stable
builds require `--pre` to enter that channel.

After confirmation, Konen downloads and validates its own candidate before any
component changes, asks the co-installed mise to update itself to the exact
planned version with plugin updates disabled, and atomically replaces Konen
last. It never mutates state repositories. Package-managed executables outside
the supported user-local installation remain manual actions in the plan.

Metadata mirrors used by integration tests can override
`KONEN_RELEASE_API_URL` and `KONEN_MISE_RELEASE_API_URL`.

## Trust model

Checksums protect against corruption and inconsistent assets, but a checksum
served from the same compromised release is not an independent signature. For a
more conservative installation, download and inspect `install.sh`, verify the
GitHub artifact attestation with `gh attestation verify`, and then execute the
installer locally.

The installer never runs Konen as root and does not edit shell startup files.
For a private `github:OWNER/REPO` state, the later interactive `konen init`
step may use mise to download an ad-hoc GitHub CLI tool. Mise may cache that
tool, but it is not silently added to the user's declared machine state. Konen
explains the action before starting device-code authentication and never
receives or stores the resulting token itself. Git invokes the CLI as a
command-scoped credential helper for the private clone; the persistent helper
entry is local to that repository, and global Git configuration is untouched.

## Later packaging

The first public distribution target is the release archive and HTTPS installer.
A `.deb` can reuse the same static executable once the CLI and configuration
contract are stable. A signed APT repository comes after that; package maintainer
scripts will not contain the interactive first-run flow.
