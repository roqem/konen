#!/usr/bin/env bash

set -euo pipefail

version="${1:-}"
if [[ "$version" != v* ]]; then
  printf '%s\n' 'uso: scripts/smoke-release.sh vVERSÃO' >&2
  exit 2
fi

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
smoke_root=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/konen-public-smoke.XXXXXX")
trap 'rm -rf -- "$smoke_root"' EXIT INT TERM

smoke_home="$smoke_root/home"
smoke_config="$smoke_home/.config"
smoke_tmp="$smoke_root/tmp"
guard_bin="$smoke_root/guard-bin"
sudo_marker="$smoke_root/sudo-was-called"
mkdir -p "$smoke_home" "$smoke_config" "$smoke_tmp" "$guard_bin"

printf '%s\n' \
  '#!/bin/sh' \
  ': > "$KONEN_SMOKE_SUDO_MARKER"' \
  'printf "%s\n" "smoke: sudo não deveria ser chamado" >&2' \
  'exit 97' \
  > "$guard_bin/sudo"
chmod 0755 "$guard_bin/sudo"

smoke_path="$smoke_home/.local/bin:$guard_bin:/usr/local/bin:/usr/bin:/bin"
smoke_env=(
  env
  "HOME=$smoke_home"
  "XDG_CONFIG_HOME=$smoke_config"
  "TMPDIR=$smoke_tmp"
  "PATH=$smoke_path"
  "SHELL=/bin/bash"
  "KONEN_SMOKE_SUDO_MARKER=$sudo_marker"
)

(
  cd "$repository_root"
  "${smoke_env[@]}" KONEN_VERSION="$version" sh install.sh
)

konen="$smoke_home/.local/bin/konen"
mise="$smoke_home/.local/bin/mise"
[[ -x "$konen" ]]
[[ -x "$mise" ]]

expected_version=${version#v}
actual_version=$("${smoke_env[@]}" "$konen" version)
[[ "$actual_version" == "$expected_version" ]]

state_dir="$smoke_home/state"
"${smoke_env[@]}" "$konen" init --git "$state_dir"
"${smoke_env[@]}" "$konen" doctor
"${smoke_env[@]}" "$konen" status
"${smoke_env[@]}" "$konen" plan --only dotfiles
"${smoke_env[@]}" "$konen" apply --only dotfiles --yes
"${smoke_env[@]}" "$konen" apply --only dotfiles --yes

ready_status=$("${smoke_env[@]}" "$konen" status --only dotfiles --state ready)
grep -F 'aplicado' <<<"$ready_status"
[[ -L "$smoke_config/mise/config.toml" ]]
[[ ! -e "$sudo_marker" ]]

if git -C "$state_dir" rev-parse --verify HEAD >/dev/null 2>&1; then
  printf '%s\n' 'smoke: o Konen criou um commit inesperado' >&2
  exit 1
fi
[[ -z "$(git -C "$state_dir" remote)" ]]

printf 'Smoke público concluído: Konen %s em HOME descartável, sem sudo.\n' "$actual_version"
