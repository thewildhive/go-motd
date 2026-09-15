#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
arch="${2:-}"
output_dir="${3:-dist}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ || ! "$arch" =~ ^(amd64|arm64)$ ]]; then
  echo "usage: $0 MAJOR.MINOR.PATCH amd64|arm64 [output-directory]" >&2
  exit 2
fi

export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD)}"
build_date="${BUILDDATE:-$(date -u -d "@$SOURCE_DATE_EPOCH" +%d%m%y)}"
mkdir -p "$output_dir"
package="$output_dir/go-motd_${version}-1_${arch}.deb"
[[ ! -e "$package" ]] || { echo "refusing to overwrite $package" >&2; exit 1; }
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
chmod 755 "$stage"
mkdir -p "$stage/DEBIAN" "$stage/usr/bin" "$stage/usr/share/doc/go-motd"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -buildvcs=false -trimpath \
  -ldflags="-s -w -X main.VERSION=$version -X main.BUILDDATE=$build_date -X main.DISTRIBUTION=deb" \
  -o "$stage/usr/bin/motd" .
install -m 0644 LICENSE "$stage/usr/share/doc/go-motd/copyright"
install -m 0644 config.json.sample "$stage/usr/share/doc/go-motd/config.json.sample"
cat > "$stage/DEBIAN/control" <<EOF
Package: go-motd
Version: ${version}-1
Section: utils
Priority: optional
Architecture: $arch
Maintainer: thewildhive <thewildhive@users.noreply.github.com>
Homepage: https://github.com/thewildhive/go-motd
Installed-Size: $(du -sk "$stage/usr" | cut -f1)
Description: System and media service statistics for your terminal
 Displays a configurable message of the day with optional media integrations.
 Configuration is user-owned; this package does not install login hooks.
EOF
find "$stage" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +
dpkg-deb --root-owner-group -Zxz --build "$stage" "$package"
