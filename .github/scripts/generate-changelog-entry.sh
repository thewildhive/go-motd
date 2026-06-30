#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "usage: $0 <previous-ref-or-empty> <target-ref> <release-tag> <date>" >&2
}

is_conventional_entry() {
  case "$1" in
    feat:\ *|fix:\ *|perf:\ *|refactor:\ *|build:\ *|docs:\ *) return 0 ;;
    feat\(*\):\ *|fix\(*\):\ *|perf\(*\):\ *|refactor\(*\):\ *|build\(*\):\ *|docs\(*\):\ *) return 0 ;;
    *) return 1 ;;
  esac
}

normalize_body_entry() {
  local line="$1"

  case "$line" in
    '- '*) line="${line#- }" ;;
    '* '*) line="${line#\* }" ;;
  esac

  if is_conventional_entry "$line"; then
    printf '%s\n' "$line"
  fi
}

emit_commit_entries() {
  local commit="$1"
  local subject
  local short_sha
  local body

  subject=$(git show -s --format=%s "$commit")
  short_sha=$(git rev-parse --short "$commit")

  if is_conventional_entry "$subject"; then
    printf -- '- %s (%s)\n' "$subject" "$short_sha"
    return
  fi

  body=$(git show -s --format=%b "$commit")
  while IFS= read -r line; do
    normalize_body_entry "$line" | while IFS= read -r entry; do
      printf -- '- %s (%s)\n' "$entry" "$short_sha"
    done
  done <<< "$body"
}

main() {
  if [ "$#" -ne 4 ]; then
    usage
    exit 2
  fi

  local previous_ref="$1"
  local target_ref="$2"
  local release_tag="$3"
  local release_date="$4"
  local range

  printf '## [%s] - %s\n\n' "$release_tag" "$release_date"

  if [ -n "$previous_ref" ]; then
    range="${previous_ref}..${target_ref}"
  else
    range="$target_ref"
  fi

  git rev-list --reverse --no-merges "$range" 2>/dev/null | while IFS= read -r commit; do
    emit_commit_entries "$commit"
  done

  printf '\n'
}

main "$@"
