#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
GENERATOR="$SCRIPT_DIR/generate-changelog-entry.sh"

assert_contains() {
  local haystack="$1"
  local needle="$2"

  if [[ "$haystack" != *"$needle"* ]]; then
    printf 'expected output to contain:\n%s\n\nactual output:\n%s\n' "$needle" "$haystack" >&2
    exit 1
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"

  if [[ "$haystack" == *"$needle"* ]]; then
    printf 'expected output not to contain:\n%s\n\nactual output:\n%s\n' "$needle" "$haystack" >&2
    exit 1
  fi
}

make_commit() {
  local subject="$1"
  local body="${2:-}"

  if [ -n "$body" ]; then
    git commit --allow-empty -m "$subject" -m "$body" >/dev/null
  else
    git commit --allow-empty -m "$subject" >/dev/null
  fi
}

repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT

cd "$repo"
git init -q
git config user.name test
git config user.email test@example.com

make_commit 'chore: initial release'
git tag v1.0.0
make_commit 'fix: keep conventional subjects'
subject_sha=$(git rev-parse --short HEAD)
make_commit 'Security and reliability remediation from review roadmap (#43)' $'fix: reject remote plaintext media service URLs\n- fix: bound media service checks\n* build: pin release actions\nThis prose mentions fix: but should not match.'
squash_sha=$(git rev-parse --short HEAD)
make_commit 'Update docs manually' $'This body has prose only.\nIt should not emit changelog content.'
noise_sha=$(git rev-parse --short HEAD)

output=$(bash "$GENERATOR" v1.0.0 HEAD v1.0.1 2026-07-01)

assert_contains "$output" '## [v1.0.1] - 2026-07-01'
assert_contains "$output" "- fix: keep conventional subjects ($subject_sha)"
assert_contains "$output" "- fix: reject remote plaintext media service URLs ($squash_sha)"
assert_contains "$output" "- fix: bound media service checks ($squash_sha)"
assert_contains "$output" "- build: pin release actions ($squash_sha)"
assert_not_contains "$output" "Update docs manually ($noise_sha)"
assert_not_contains "$output" 'This prose mentions fix:'
