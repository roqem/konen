#!/bin/sh

set -eu

repository="${KONEN_REPOSITORY:-roqem/konen}"
release_base="${KONEN_RELEASE_BASE_URL:-https://github.com/$repository/releases}"
mise_release_base="${KONEN_MISE_RELEASE_BASE_URL:-https://github.com/jdx/mise/releases}"
mise_version="${KONEN_MISE_VERSION:-2026.8.15}"
install_mise="${KONEN_INSTALL_MISE:-1}"

say() {
  printf '%s\n' "$*"
}

fail() {
  printf 'konen-install: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "comando necessário não encontrado: $1"
}

download() {
  source_url=$1
  destination=$2
  if ! curl -fL --retry 3 --retry-all-errors --retry-delay 1 \
    --connect-timeout 15 --max-time 300 \
    "$source_url" -o "$destination"; then
    fail "não foi possível baixar $source_url"
  fi
}

checksum_for() {
  checksum_file=$1
  asset_name=$2
  awk -v name="$asset_name" '{
    entry = $2
    sub(/^\*/, "", entry)
    sub(/^\.\//, "", entry)
    if (entry == name) {
      print $1
      exit
    }
  }' "$checksum_file"
}

verify_checksum() {
  file_path=$1
  checksum_file=$2
  asset_name=$3
  expected=$(checksum_for "$checksum_file" "$asset_name")
  [ -n "$expected" ] || fail "checksum não encontrado para $asset_name"
  actual=$(sha256sum "$file_path" | awk '{ print $1 }')
  [ "$actual" = "$expected" ] || fail "checksum inválido para $asset_name"
}

version_at_least() {
  current=$1
  required=$2
  awk -v current="$current" -v required="$required" 'BEGIN {
    split(current, a, ".")
    split(required, b, ".")
    for (i = 1; i <= 3; i++) {
      a[i] += 0
      b[i] += 0
      if (a[i] > b[i]) exit 0
      if (a[i] < b[i]) exit 1
    }
    exit 0
  }'
}

installed_mise() {
  if [ -x "$install_dir/mise" ]; then
    printf '%s\n' "$install_dir/mise"
  elif command -v mise >/dev/null 2>&1; then
    command -v mise
  fi
}

installed_mise_version() {
  mise_binary=$1
  "$mise_binary" --version 2>/dev/null | awk 'NR == 1 {
    for (i = 1; i <= NF; i++) {
      value = $i
      sub(/^v/, "", value)
      if (split(value, parts, ".") == 3 && parts[1] ~ /^[0-9]+$/ && parts[2] ~ /^[0-9]+$/ && parts[3] ~ /^[0-9]+$/) {
        print value
        exit
      }
    }
  }'
}

require_command curl
require_command tar
require_command awk
require_command sha256sum
require_command install
require_command mktemp
require_command mkdir
require_command mv
require_command rm
require_command uname

[ "$(uname -s)" = "Linux" ] || fail "esta versão do instalador suporta somente Linux"

case "$(uname -m)" in
  x86_64 | amd64)
    architecture=amd64
    mise_architecture=x64
    ;;
  aarch64 | arm64)
    architecture=arm64
    mise_architecture=arm64
    ;;
  *)
    fail "arquitetura não suportada: $(uname -m)"
    ;;
esac

if [ -n "${KONEN_INSTALL_DIR:-}" ]; then
  install_dir=$KONEN_INSTALL_DIR
else
  [ -n "${HOME:-}" ] || fail "HOME não está definido; use KONEN_INSTALL_DIR"
  install_dir="$HOME/.local/bin"
fi

case "$install_dir" in
  /*) ;;
  *) install_dir="$PWD/$install_dir" ;;
esac

if [ -n "${KONEN_VERSION:-}" ]; then
  version=${KONEN_VERSION#v}
else
  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$release_base/latest")
  latest_tag=${latest_url##*/}
  version=${latest_tag#v}
fi

case "$version" in
  "" | *[!0-9A-Za-z._-]*) fail "versão inválida: $version" ;;
esac

temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/konen-install.XXXXXX")
staged_konen=""
staged_mise=""
staged_konen_dir=""
staged_mise_dir=""
cleanup() {
  rm -rf -- "$temporary_dir"
  [ -z "$staged_konen_dir" ] || rm -rf -- "$staged_konen_dir"
  [ -z "$staged_mise_dir" ] || rm -rf -- "$staged_mise_dir"
}
trap cleanup 0 1 2 15

archive="konen_${version}_linux_${architecture}.tar.gz"
archive_path="$temporary_dir/$archive"
checksums_path="$temporary_dir/checksums.txt"
download_root="$release_base/download/v$version"

say "Baixando Konen $version para linux/$architecture..."
download "$download_root/$archive" "$archive_path"
download "$download_root/checksums.txt" "$checksums_path"
verify_checksum "$archive_path" "$checksums_path" "$archive"

mkdir -p "$temporary_dir/konen"
if ! tar -xzf "$archive_path" -C "$temporary_dir/konen" konen; then
  fail "não foi possível extrair o executável konen do archive"
fi
[ -f "$temporary_dir/konen/konen" ] && [ ! -L "$temporary_dir/konen/konen" ] ||
  fail "o archive não contém um executável konen regular"

mise_was_installed=0
if [ "$install_mise" != "0" ]; then
  mise_path=$(installed_mise || true)
  current_mise_version=""
  if [ -n "$mise_path" ]; then
    current_mise_version=$(installed_mise_version "$mise_path" || true)
  fi

  if [ -n "$current_mise_version" ] && version_at_least "$current_mise_version" "$mise_version"; then
    say "mise $current_mise_version já atende ao mínimo $mise_version."
  else
    mise_asset="mise-v${mise_version}-linux-${mise_architecture}"
    mise_path_tmp="$temporary_dir/$mise_asset"
    mise_checksums="$temporary_dir/mise-checksums.txt"
    mise_download_root="$mise_release_base/download/v$mise_version"

    say "Instalando mise $mise_version..."
    download "$mise_download_root/$mise_asset" "$mise_path_tmp"
    download "$mise_download_root/SHASUMS256.txt" "$mise_checksums"
    verify_checksum "$mise_path_tmp" "$mise_checksums" "$mise_asset"
    mise_was_installed=1
  fi
fi

install -d "$install_dir"
[ ! -d "$install_dir/konen" ] || fail "$install_dir/konen é um diretório"

staged_konen_dir=$(mktemp -d "$install_dir/.konen-install.XXXXXX")
staged_konen="$staged_konen_dir/konen"
install -m 0755 "$temporary_dir/konen/konen" "$staged_konen"
staged_konen_version=$("$staged_konen" version 2>/dev/null || true)
staged_konen_version=${staged_konen_version#v}
[ "$staged_konen_version" = "$version" ] ||
  fail "o executável baixado informou ${staged_konen_version:-uma versão inválida}; esperado $version"

if [ "$mise_was_installed" = "1" ]; then
  [ ! -d "$install_dir/mise" ] || fail "$install_dir/mise é um diretório"
  staged_mise_dir=$(mktemp -d "$install_dir/.mise-install.XXXXXX")
  staged_mise="$staged_mise_dir/mise"
  install -m 0755 "$mise_path_tmp" "$staged_mise"
  staged_mise_version=$(installed_mise_version "$staged_mise" || true)
  [ "$staged_mise_version" = "$mise_version" ] ||
    fail "o executável do mise informou ${staged_mise_version:-uma versão inválida}; esperado $mise_version"

  mv -f -- "$staged_mise" "$install_dir/mise"
  staged_mise=""
fi
mv -f -- "$staged_konen" "$install_dir/konen"
staged_konen=""

say "Konen $version instalado em $install_dir/konen"
if [ "$mise_was_installed" = "1" ]; then
  say "mise $mise_version instalado em $install_dir/mise"
fi

case ":${PATH:-}:" in
  *":$install_dir:"*)
    say "Execute: konen"
    ;;
  *)
    say "Adicione $install_dir ao PATH ou execute: $install_dir/konen"
    ;;
esac

case "${SHELL:-}" in
  */zsh) say 'Autocomplete: adicione `eval "$(konen completion zsh)"` ao ~/.zshrc.' ;;
  */bash) say 'Autocomplete: adicione `eval "$(konen completion bash)"` ao ~/.bashrc.' ;;
  */fish) say 'Autocomplete: execute `konen completion fish > ~/.config/fish/completions/konen.fish`.' ;;
esac
