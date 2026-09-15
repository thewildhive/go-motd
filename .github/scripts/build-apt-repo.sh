#!/usr/bin/env bash
# Input packages must already have been verified against signed release manifests.
set -euo pipefail
packages="${1:?package directory required}"
output="${2:?empty output directory required}"
: "${APT_SIGNING_FINGERPRINT:?signing fingerprint required}"
[[ "$APT_SIGNING_FINGERPRINT" =~ ^[A-F0-9]{40}$ ]]
mkdir -p "$output"
[[ -z "$(find "$output" -mindepth 1 -print -quit)" ]]
mkdir -p "$output/pool/main/g/go-motd"
shopt -s nullglob
assets=("$packages"/*.deb)
[[ ${#assets[@]} -gt 0 ]]
for package in "${assets[@]}"; do
  name="$(basename "$package")"
  [[ "$name" =~ ^go-motd_([0-9]+\.[0-9]+\.[0-9]+)-1_(amd64|arm64)\.deb$ ]]
  version="${BASH_REMATCH[1]}"
  arch="${BASH_REMATCH[2]}"
  [[ "$(dpkg-deb -f "$package" Package)" == go-motd ]]
  [[ "$(dpkg-deb -f "$package" Version)" == "$version-1" ]]
  [[ "$(dpkg-deb -f "$package" Architecture)" == "$arch" ]]
  install -m 0644 "$package" "$output/pool/main/g/go-motd/$name"
done
(
  cd "$output"
  for arch in amd64 arm64; do
    directory="dists/stable/main/binary-$arch"
    mkdir -p "$directory"
    dpkg-scanpackages --multiversion --arch "$arch" pool /dev/null > "$directory/Packages"
    [[ -s "$directory/Packages" ]]
    gzip -n -9 -c "$directory/Packages" > "$directory/Packages.gz"
  done
  apt-ftparchive \
    -o APT::FTPArchive::Release::Origin=go-motd \
    -o APT::FTPArchive::Release::Label=go-motd \
    -o APT::FTPArchive::Release::Suite=stable \
    -o APT::FTPArchive::Release::Codename=stable \
    -o 'APT::FTPArchive::Release::Architectures=amd64 arm64' \
    -o APT::FTPArchive::Release::Components=main \
    release dists/stable > Release.tmp
  # Expiry limits replay of old signed metadata. The workflow refreshes weekly.
  printf 'Valid-Until: %s\n' "$(date -u -R -d '+30 days')" >> Release.tmp
  mv Release.tmp dists/stable/Release
  gpg --batch --yes --local-user "$APT_SIGNING_FINGERPRINT" --digest-algo SHA256 \
    --clearsign -o dists/stable/InRelease dists/stable/Release
  gpg --batch --yes --local-user "$APT_SIGNING_FINGERPRINT" --digest-algo SHA256 \
    --armor --detach-sign -o dists/stable/Release.gpg dists/stable/Release
  gpg --batch --export "$APT_SIGNING_FINGERPRINT" > go-motd-archive-keyring.gpg
  gpgv --keyring "$PWD/go-motd-archive-keyring.gpg" dists/stable/InRelease
  gpgv --keyring "$PWD/go-motd-archive-keyring.gpg" dists/stable/Release.gpg dists/stable/Release
  printf '%s\n' "$APT_SIGNING_FINGERPRINT" > signing-key-fingerprint.txt
  touch .nojekyll
)
