#!/usr/bin/env bash
# determine-version.sh
#
# Determines the next semantic version based on conventional commits
# since the last Git tag. Uses svu for the core logic with overrides
# to match the project's release rules defined in CONTRIBUTING.md.
#
# Only commit types that change the compiled binary trigger releases:
#   feat      -> minor   (new functionality)
#   fix       -> patch   (bug fixes)
#   perf      -> minor   (performance improvements)
#   refactor  -> patch   (code restructuring that changes the binary)
#   build     -> patch   (dependency changes, compiler flags)
#   BREAKING  -> major
#
# Types that do NOT affect the binary skip releases:
#   docs, style, test, ci, chore
#
# Outputs "version=vX.Y.Z" to GITHUB_OUTPUT on success, or
# "version=" (empty) when no release should be made.

set -euo pipefail

bump_patch_version() {
  local tag="$1"
  local version="${tag#v}"

  if ! echo "$version" | grep -Eq '^([0-9]+\.){2}[0-9]+$'; then
    echo ""
    return
  fi

  local major
  local rest
  local minor
  local patch

  major=${version%%.*}
  rest=${version#*.}
  minor=${rest%%.*}
  patch=${version##*.}
  patch=$((patch + 1))

  printf 'v%s.%s.%s\n' "$major" "$minor" "$patch"
}

resolve_version_collision() {
  local tag="$1"
  local head
  local existing
  local next_tag
  local max_attempts=128
  local attempt=0

  head=$(git rev-parse HEAD)

  while true; do
    if ! git rev-parse -q --verify "refs/tags/${tag}^{commit}" >/dev/null 2>&1; then
      printf '%s' "$tag"
      return
    fi

    existing=$(git rev-parse "${tag}^{commit}")
    if [ "$existing" = "$head" ]; then
      printf ''
      return
    fi

    next_tag=$(bump_patch_version "$tag")
    if [ -z "$next_tag" ]; then
      echo "warning: could not advance non-semver collision tag '${tag}', skipping release" >&2
      printf ''
      return
    fi

    attempt=$((attempt + 1))
    if [ "$attempt" -gt "$max_attempts" ]; then
      echo "warning: exceeded ${max_attempts} collision increments for ${tag}, skipping release" >&2
      printf ''
      return
    fi

    tag="$next_tag"
  done
}

CURRENT_TAG=$(svu current 2>/dev/null || echo "v0.0.0")

# No tags yet — first release
if [ "$CURRENT_TAG" = "v0.0.0" ]; then
  NEXT_TAG=$(svu minor)
  echo "version=$(resolve_version_collision "$NEXT_TAG")"
  exit 0
fi

# If no commits since the last tag, nothing to release
COMMIT_COUNT=$(git rev-list --count "$CURRENT_TAG..HEAD" 2>/dev/null || echo "0")
if [ "$COMMIT_COUNT" -eq 0 ]; then
  echo "version="
  exit 0
fi

# --- Step 1: Let svu handle the standard conventional commit types ---
# svu natively handles: feat -> minor, fix -> patch, BREAKING -> major
NEXT_TAG=$(svu next 2>/dev/null || echo "")

if [ -n "$NEXT_TAG" ] && [ "$NEXT_TAG" != "$CURRENT_TAG" ]; then
  echo "version=$(resolve_version_collision "$NEXT_TAG")"
  exit 0
fi

# --- Step 2: Handle types that affect the binary but svu ignores ---
# svu does not bump for perf, refactor, or build by default.
# Collect conventional commit types since last tag, excluding all
# non-binary-affecting types (docs, style, test, ci, chore).

RELEASE_TYPES=$(git log "$CURRENT_TAG..HEAD" --no-merges --format="%s" 2>/dev/null | \
  sed -n 's/^\([a-z]*\)\(!\|(\S*)\)\{0,1\}:.*/\1/p' | \
  grep -E '^(feat|fix|perf|refactor|build)$' || true)

if [ -z "$RELEASE_TYPES" ]; then
  # No commits affecting the binary — skip release
  echo "version="
  exit 0
fi

# Check for breaking changes (look for ! after type/scope or BREAKING CHANGE in body)
if git log "$CURRENT_TAG..HEAD" --no-merges --format="%s%n%b" 2>/dev/null | \
     grep -q -E '(BREAKING[ -]CHANGE|^[a-z]+\(.*\)!:|^[a-z]+!:)'; then
  version=$(svu major)
  echo "version=$(resolve_version_collision "$version")"
  exit 0
fi

# Check for feat or perf -> minor
if echo "$RELEASE_TYPES" | grep -q -E '^(feat|perf)$'; then
  version=$(svu minor)
  echo "version=$(resolve_version_collision "$version")"
  exit 0
fi

# fix, refactor, build -> patch
version=$(svu patch)
echo "version=$(resolve_version_collision "$version")"
