#!/usr/bin/env bash

set -euo pipefail

version="${1:-}"
output_dir="${2:-dist}"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: $0 MAJOR.MINOR.PATCH [empty-output-directory]" >&2
  exit 2
fi
if [[ -z "$output_dir" || "$output_dir" == / ]]; then
  echo "refusing unsafe output directory: $output_dir" >&2
  exit 2
fi

mkdir -p "$output_dir"
if [[ -n "$(find "$output_dir" -mindepth 1 -print -quit)" ]]; then
  echo "output directory must be empty: $output_dir" >&2
  exit 2
fi

source_date_epoch="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD)}"
build_date="${BUILDDATE:-$(date -u -d "@$source_date_epoch" +%d%m%y)}"
ldflags="-s -w -X main.VERSION=$version -X main.BUILDDATE=$build_date"
export SOURCE_DATE_EPOCH="$source_date_epoch"

build() {
  local goos="$1"
  local goarch="$2"
  local filename="motd-${goos}-${goarch}"
  if [[ "$goos" == windows ]]; then
    filename+=".exe"
  fi
  GOOS="$goos" GOARCH="$goarch" go build -buildvcs=false -trimpath -ldflags="$ldflags" -o "$output_dir/$filename" .
}

build linux amd64
build linux arm64
build darwin amd64
build darwin arm64
build windows amd64

touch -d "@$source_date_epoch" \
  "$output_dir/motd-linux-amd64" \
  "$output_dir/motd-linux-arm64" \
  "$output_dir/motd-darwin-amd64" \
  "$output_dir/motd-darwin-arm64" \
  "$output_dir/motd-windows-amd64.exe"

(
  cd "$output_dir"
  sha256sum \
    motd-linux-amd64 \
    motd-linux-arm64 \
    motd-darwin-amd64 \
    motd-darwin-arm64 \
    motd-windows-amd64.exe > checksums.txt

  tar_flags=(--sort=name --mtime="@$source_date_epoch" --owner=0 --group=0 --numeric-owner)
  tar "${tar_flags[@]}" -czf "motd-${version}-linux-amd64.tar.gz" motd-linux-amd64
  tar "${tar_flags[@]}" -czf "motd-${version}-linux-arm64.tar.gz" motd-linux-arm64
  tar "${tar_flags[@]}" -czf "motd-${version}-darwin-amd64.tar.gz" motd-darwin-amd64
  tar "${tar_flags[@]}" -czf "motd-${version}-darwin-arm64.tar.gz" motd-darwin-arm64
  TZ=UTC python3 -m zipfile -c "motd-${version}-windows-amd64.zip" motd-windows-amd64.exe
  sha256sum *.tar.gz *.zip > archive-checksums.txt
)

if [[ -z "${SIGNING_KEY_FILE:-}" || ! -s "$SIGNING_KEY_FILE" ]]; then
  echo "SIGNING_KEY_FILE must name a readable Ed25519 private key" >&2
  exit 2
fi

openssl pkeyutl -sign -inkey "$SIGNING_KEY_FILE" -rawin -in "$output_dir/checksums.txt" -out "$output_dir/checksums.txt.sig"
openssl pkeyutl -sign -inkey "$SIGNING_KEY_FILE" -rawin -in "$output_dir/archive-checksums.txt" -out "$output_dir/archive-checksums.txt.sig"

(
  cd "$output_dir"
  sha256sum --check checksums.txt
  sha256sum --check archive-checksums.txt
  [[ "$(wc -l < checksums.txt)" -eq 5 ]]
  [[ "$(wc -l < archive-checksums.txt)" -eq 5 ]]
  [[ "$(wc -c < checksums.txt.sig)" -eq 64 ]]
  [[ "$(wc -c < archive-checksums.txt.sig)" -eq 64 ]]

  host_os="$(go env GOOS)"
  host_arch="$(go env GOARCH)"
  smoke_binary="motd-${host_os}-${host_arch}"
  if [[ "$host_os" == windows ]]; then
    smoke_binary+=".exe"
  fi
  if [[ ! -x "$smoke_binary" ]]; then
    echo "no runnable release target for ${host_os}/${host_arch}" >&2
    exit 1
  fi
  "./$smoke_binary" -v | grep -F "v${version}"
  "./$smoke_binary" -h >/dev/null
)
