#!/bin/sh

set -eu

repository="${ZEROOT_REPOSITORY:-roqem/zeroot}"
release_base="${ZEROOT_RELEASE_BASE_URL:-https://github.com/$repository/releases}"
mise_release_base="${ZEROOT_MISE_RELEASE_BASE_URL:-https://github.com/jdx/mise/releases}"
mise_version="${ZEROOT_MISE_VERSION:-2026.8.14}"
install_mise="${ZEROOT_INSTALL_MISE:-1}"

say() {
  printf '%s\n' "$*"
}

fail() {
  printf 'zeroot-install: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "comando necessário não encontrado: $1"
}

download() {
  source_url=$1
  destination=$2
  curl -fL --retry 3 --retry-delay 1 --connect-timeout 15 \
    "$source_url" -o "$destination"
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

if [ -n "${ZEROOT_INSTALL_DIR:-}" ]; then
  install_dir=$ZEROOT_INSTALL_DIR
else
  [ -n "${HOME:-}" ] || fail "HOME não está definido; use ZEROOT_INSTALL_DIR"
  install_dir="$HOME/.local/bin"
fi

case "$install_dir" in
  /*) ;;
  *) install_dir="$PWD/$install_dir" ;;
esac

if [ -n "${ZEROOT_VERSION:-}" ]; then
  version=${ZEROOT_VERSION#v}
else
  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$release_base/latest")
  latest_tag=${latest_url##*/}
  version=${latest_tag#v}
fi

case "$version" in
  "" | *[!0-9A-Za-z._-]*) fail "versão inválida: $version" ;;
esac

temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/zeroot-install.XXXXXX")
trap 'rm -rf -- "$temporary_dir"' 0 1 2 15

archive="zeroot_${version}_linux_${architecture}.tar.gz"
archive_path="$temporary_dir/$archive"
checksums_path="$temporary_dir/checksums.txt"
download_root="$release_base/download/v$version"

say "Baixando Zeroot $version para linux/$architecture..."
download "$download_root/$archive" "$archive_path"
download "$download_root/checksums.txt" "$checksums_path"
verify_checksum "$archive_path" "$checksums_path" "$archive"

mkdir -p "$temporary_dir/zeroot"
tar -xzf "$archive_path" -C "$temporary_dir/zeroot"
[ -f "$temporary_dir/zeroot/zeroot" ] || fail "o archive não contém o executável zeroot"

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
install -m 0755 "$temporary_dir/zeroot/zeroot" "$install_dir/zeroot"
if [ "$mise_was_installed" = "1" ]; then
  install -m 0755 "$mise_path_tmp" "$install_dir/mise"
fi

say "Zeroot $version instalado em $install_dir/zeroot"
if [ "$mise_was_installed" = "1" ]; then
  say "mise $mise_version instalado em $install_dir/mise"
fi

case ":${PATH:-}:" in
  *":$install_dir:"*)
    say "Execute: zeroot"
    ;;
  *)
    say "Adicione $install_dir ao PATH ou execute: $install_dir/zeroot"
    ;;
esac
