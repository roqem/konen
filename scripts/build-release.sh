#!/usr/bin/env bash

set -euo pipefail

version="${1:-}"
version="${version#v}"

if [[ $# -gt 2 ]]; then
  echo "uso: scripts/build-release.sh <versão> [diretório de saída]" >&2
  exit 2
fi

case "$version" in
  "" | *[!0-9A-Za-z._-]*)
    echo "uso: scripts/build-release.sh <versão> [diretório de saída]" >&2
    exit 2
    ;;
esac

for required_command in go git tar gzip sha256sum install find; do
  if ! command -v "$required_command" >/dev/null 2>&1; then
    echo "comando necessário não encontrado: $required_command" >&2
    exit 1
  fi
done

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist_dir="${2:-$repository_root/dist}"
if [[ "$dist_dir" != /* ]]; then
  dist_dir="$repository_root/$dist_dir"
fi

if [[ -d "$dist_dir" ]] && [[ -n "$(find "$dist_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  echo "dist/ não está vazio; preserve ou remova os artefatos existentes antes de continuar" >&2
  exit 1
fi

mkdir -p "$dist_dir"
build_root=$(mktemp -d "${TMPDIR:-/tmp}/konen-release.XXXXXX")
trap 'rm -rf -- "$build_root"' EXIT INT TERM

source_date_epoch="${SOURCE_DATE_EPOCH:-$(git -C "$repository_root" log -1 --format=%ct)}"

for architecture in amd64 arm64; do
  stage_dir="$build_root/$architecture"
  mkdir -p "$stage_dir"

  (
    cd "$repository_root"
    CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" \
      go build \
        -buildvcs=false \
        -trimpath \
        -ldflags="-s -w -buildid= -X main.version=$version" \
        -o "$stage_dir/konen" \
        ./cmd/konen
  )

  install -m 0644 "$repository_root/README.md" "$stage_dir/README.md"
  install -m 0644 "$repository_root/LICENSE" "$stage_dir/LICENSE"

  archive="konen_${version}_linux_${architecture}.tar.gz"
  tar \
    --sort=name \
    --mtime="@$source_date_epoch" \
    --owner=0 \
    --group=0 \
    --numeric-owner \
    -C "$stage_dir" \
    -cf - \
    LICENSE README.md konen | gzip -n -9 > "$dist_dir/$archive"
done

(
  cd "$dist_dir"
  sha256sum konen_*.tar.gz > checksums.txt
)

echo "Artefatos gerados em $dist_dir"
