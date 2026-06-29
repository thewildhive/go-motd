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

CURRENT_TAG=$(svu current 2>/dev/null || echo "v0.0.0")

# No tags yet — first release
if [ "$CURRENT_TAG" = "v0.0.0" ]; then
  echo "version=$(svu minor)"
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
  echo "version=$NEXT_TAG"
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
  echo "version=$(svu major)"
  exit 0
fi

# Check for feat or perf -> minor
if echo "$RELEASE_TYPES" | grep -q -E '^(feat|perf)$'; then
  echo "version=$(svu minor)"
  exit 0
fi

# fix, refactor, build -> patch
echo "version=$(svu patch)"
