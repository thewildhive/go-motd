#!/usr/bin/env bash
# Rebuild from immutable, signed release assets rather than trusting prior Pages data.
set -euo pipefail
output="${1:?empty package output directory required}"
repository=thewildhive/go-motd
mkdir -p "$output"
[[ -z "$(find "$output" -mindepth 1 -print -quit)" ]]
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
sed -n '/^-----BEGIN PUBLIC KEY-----$/,/^-----END PUBLIC KEY-----$/p' install-deb.sh > "$work/public.pem"
gh api --paginate "repos/$repository/releases?per_page=100" \
  --jq '.[] | select(.draft == false and .prerelease == false) | .tag_name' > "$work/tags"
while IFS= read -r tag; do
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
  version="${tag#v}"
  dpkg --compare-versions "$version" ge 3.0.0 || continue
  directory="$work/$tag"
  mkdir "$directory"
  gh release download "$tag" --repo "$repository" --dir "$directory" \
    --pattern archive-checksums.txt --pattern archive-checksums.txt.sig \
    --pattern "go-motd_${version}-1_amd64.deb" --pattern "go-motd_${version}-1_arm64.deb"
  openssl pkeyutl -verify -pubin -inkey "$work/public.pem" -rawin \
    -in "$directory/archive-checksums.txt" -sigfile "$directory/archive-checksums.txt.sig"
  for arch in amd64 arm64; do
    asset="go-motd_${version}-1_${arch}.deb"
    expected="$(awk -v name="$asset" '$2 == name {print $1}' "$directory/archive-checksums.txt")"
    [[ "$expected" =~ ^[0-9a-f]{64}$ ]]
    printf '%s  %s\n' "$expected" "$directory/$asset" | sha256sum --check --strict
    install -m 0644 "$directory/$asset" "$output/$asset"
  done
done < "$work/tags"
[[ -n "$(find "$output" -name '*.deb' -print -quit)" ]]
