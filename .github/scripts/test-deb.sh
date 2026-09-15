#!/usr/bin/env bash
# Uses a disposable dpkg database/root; never installs into the host system.
set -euo pipefail
export PATH="$PATH:/usr/sbin:/sbin"
arch="$(dpkg --print-architecture)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
root="$work/root"
mkdir -p "$root/home/test/.config/motd" "$root/opt/motd"
printf 'user config sentinel\n' > "$root/home/test/.config/motd/config.json"
printf 'fallback config sentinel\n' > "$root/opt/motd/config.json"
before="$(sha256sum "$root/home/test/.config/motd/config.json" "$root/opt/motd/config.json")"

for version in 2.99.0 3.0.0; do
  bash .github/scripts/package-deb.sh "$version" "$arch" "$work/packages"
  package="$work/packages/go-motd_${version}-1_${arch}.deb"
  [[ "$(dpkg-deb -f "$package" Architecture)" == "$arch" ]]
  [[ "$(dpkg-deb -f "$package" Version)" == "${version}-1" ]]
  dpkg --root="$root" --log="$work/dpkg.log" --force-not-root --install "$package"
  binary="$root/usr/bin/motd"
  "$binary" -v | grep -F "v${version}"
  "$binary" -h >/dev/null
  HOME="$root/home/test" "$binary" -no-config -json | jq -e 'type == "object"' >/dev/null
  checksum="$(sha256sum "$binary")"
  for force in '' --force; do
    if HOME="$root/home/test" "$binary" self-update ${force:+"$force"} >"$work/update.log" 2>&1; then
      echo "package self-update unexpectedly succeeded" >&2
      exit 1
    fi
    grep -F 'managed by dpkg' "$work/update.log"
    [[ "$(sha256sum "$binary")" == "$checksum" ]]
  done
  HOME="$root/home/test" "$binary" -no-config -no-color >/dev/null
  [[ ! -e "$root/home/test/.cache/motd/motd-version-check" ]]
  [[ "$(sha256sum "$root/home/test/.config/motd/config.json" "$root/opt/motd/config.json")" == "$before" ]]
done
dpkg --root="$root" --log="$work/dpkg.log" --force-not-root --remove go-motd
[[ ! -e "$root/usr/bin/motd" ]]
dpkg --root="$root" --log="$work/dpkg.log" --force-not-root --install "$package"
dpkg --root="$root" --log="$work/dpkg.log" --force-not-root --purge go-motd
[[ ! -e "$root/usr/bin/motd" ]]
[[ "$(sha256sum "$root/home/test/.config/motd/config.json" "$root/opt/motd/config.json")" == "$before" ]]
echo "Debian $arch install, upgrade, removal, config preservation, and updater checks passed."
