#!/usr/bin/env bash
# Install a signed release package; migrate the default standalone path only on request.
set -euo pipefail
version="${1:-}"
migrate="${2:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ || ( -n "$migrate" && "$migrate" != --migrate ) || $# -gt 2 ]]; then
  echo "usage: bash install-deb.sh MAJOR.MINOR.PATCH [--migrate]" >&2
  exit 2
fi
for command in curl openssl dpkg dpkg-deb dpkg-query apt sha256sum; do
  command -v "$command" >/dev/null || { echo "required command missing: $command" >&2; exit 1; }
done
arch="$(dpkg --print-architecture)"
[[ "$arch" == amd64 || "$arch" == arm64 ]] || { echo "unsupported architecture: $arch" >&2; exit 1; }
privilege=()
if [[ "$EUID" -ne 0 ]]; then
  command -v sudo >/dev/null
  privilege=(sudo)
fi
old=/usr/local/bin/motd
if [[ "$migrate" == --migrate && ( -e "$old" || -L "$old" ) ]]; then
  if [[ -L "$old" ]]; then
    [[ "$(readlink "$old")" == /usr/bin/motd ]] || { echo "refusing to change custom symlink: $old" >&2; exit 1; }
  else
    [[ -f "$old" && -x "$old" ]] || { echo "not an executable file: $old" >&2; exit 1; }
    "$old" -v | grep -q '^MOTD Script v' || { echo "unrecognized binary: $old" >&2; exit 1; }
    if dpkg-query -S "$old" >/dev/null 2>&1; then
      echo "refusing to migrate a package-owned file: $old" >&2
      exit 1
    fi
  fi
fi
if [[ -e /usr/bin/motd || -L /usr/bin/motd ]]; then
  owner="$(dpkg-query -S /usr/bin/motd 2>/dev/null || true)"
  [[ "$owner" == 'go-motd: /usr/bin/motd' ]] || { echo "refusing to overwrite an unmanaged or foreign /usr/bin/motd" >&2; exit 1; }
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
chmod 755 "$work"
asset="go-motd_${version}-1_${arch}.deb"
base="https://github.com/thewildhive/go-motd/releases/download/v${version}"
for name in "$asset" archive-checksums.txt archive-checksums.txt.sig; do
  curl --fail --show-error --location --proto '=https' --proto-redir '=https' "$base/$name" -o "$work/$name"
done
cat > "$work/public.pem" <<'EOF'
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAd8bWPCLR5lsXk1Y7SMKtABFxFPWBR2ztOLMiiwL5mDE=
-----END PUBLIC KEY-----
EOF
openssl pkeyutl -verify -pubin -inkey "$work/public.pem" -rawin \
  -in "$work/archive-checksums.txt" -sigfile "$work/archive-checksums.txt.sig"
expected="$(awk -v name="$asset" '$2 == name { print $1 }' "$work/archive-checksums.txt")"
[[ "$expected" =~ ^[0-9a-f]{64}$ ]] || { echo "missing or ambiguous package checksum" >&2; exit 1; }
printf '%s  %s\n' "$expected" "$work/$asset" | sha256sum --check --strict
[[ "$(dpkg-deb -f "$work/$asset" Package)" == go-motd ]]
[[ "$(dpkg-deb -f "$work/$asset" Version)" == "${version}-1" ]]
[[ "$(dpkg-deb -f "$work/$asset" Architecture)" == "$arch" ]]
"${privilege[@]}" apt install "$work/$asset"
/usr/bin/motd -v | grep -F "v${version} "

if [[ "$migrate" == --migrate && -f "$old" && ! -L "$old" ]]; then
  backup="$("${privilege[@]}" mktemp /usr/local/bin/motd.pre-deb.XXXXXX)"
  "${privilege[@]}" cp --preserve=mode,timestamps "$old" "$backup"
  "${privilege[@]}" ln -s /usr/bin/motd "$backup.link"
  "${privilege[@]}" mv -Tf "$backup.link" "$old"
  echo "Replaced $old with a compatibility symlink; rollback binary: $backup"
fi
echo "Installed go-motd ${version}-1 ($arch). Configuration was not changed."
echo "No APT repository is configured yet. Install future signed .deb releases with this helper."
if [[ -e "$old" && ! -L "$old" ]]; then
  echo "Warning: $old may shadow /usr/bin/motd. Re-run with --migrate to migrate this path."
fi
