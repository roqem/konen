# Distribution

## Release artifacts

A tag named `vX.Y.Z` triggers the release workflow. The workflow runs formatting,
static analysis and tests, then publishes deterministic archives for Linux
`amd64` and `arm64` plus `checksums.txt`.

Archives are built by `scripts/build-release.sh`, not by an opaque release
service. They contain only the Zeroot executable, README and Apache-2.0 license.
GitHub's build-provenance action also attests the archives.

## Installer

`install.sh` installs into `~/.local/bin` without sudo. It:

1. resolves the latest Zeroot release, unless `ZEROOT_VERSION` is set;
2. detects Linux architecture;
3. downloads the matching archive and verifies its SHA-256 checksum;
4. installs the Zeroot executable;
5. keeps a compatible mise already on `PATH`, or installs the pinned mise
   release beside Zeroot after verifying mise's official checksum.

Zeroot searches for mise beside its own executable first and on `PATH` second.
The co-installed, compatible backend therefore wins over an older system mise,
and the installation works before the user has added `~/.local/bin` to their
shell configuration.

Supported overrides:

- `ZEROOT_VERSION=vX.Y.Z`
- `ZEROOT_INSTALL_DIR=/absolute/path`
- `ZEROOT_INSTALL_MISE=0`
- `ZEROOT_MISE_VERSION=YYYY.M.PATCH`
- `ZEROOT_REPOSITORY=owner/repository`

The release-base URL overrides exist for mirrors and integration tests:
`ZEROOT_RELEASE_BASE_URL` and `ZEROOT_MISE_RELEASE_BASE_URL`.

## Trust model

Checksums protect against corruption and inconsistent assets, but a checksum
served from the same compromised release is not an independent signature. For a
more conservative installation, download and inspect `install.sh`, verify the
GitHub artifact attestation with `gh attestation verify`, and then execute the
installer locally.

The installer never runs Zeroot as root and does not edit shell startup files.

## Later packaging

The first public distribution target is the release archive and HTTPS installer.
A `.deb` can reuse the same static executable once the CLI and configuration
contract are stable. A signed APT repository comes after that; package maintainer
scripts will not contain the interactive first-run flow.
