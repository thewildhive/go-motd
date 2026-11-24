# Use mise in CI

Status: **proposed for GitHub Actions**. Local development already uses mise. Neither CI nor release publishing installs mise today.

## Current behavior

- [Local tooling](../../mise.toml) pins Go, govulncheck, actionlint, and ShellCheck. The [Makefile](../../Makefile) runs them through mise with `GOTOOLCHAIN=local`. The Go pin and [`go.mod`](../../go.mod) directive must be updated together.
- [CI](../../.github/workflows/ci.yml) uses `actions/setup-go` with `go-version-file: go.mod`. Quality runs formatting, module consistency, vet, a separately pinned actionlint, release-script syntax, PR-title validation, and offline Markdown link checks. The five cross-builds and native binary vulnerability scan are separate jobs. CI pins govulncheck independently of mise. `CI / Required` aggregates these jobs and the PR-only dependency review.
- [Release Please](../../.github/workflows/release.yml) manages the release PR, version, tag, and GitHub release. [Publish Release](../../.github/workflows/publish-release.yml) checks out the validated tag and selects Go from its `go.mod` before packaging. [APT publication](../../.github/workflows/apt-pages.yml) consumes signed Debian release assets and does not compile Go.
- [Renovate](../../renovate.json) is limited to the mise manager and `mise.toml`; it does not edit `go.mod` or workflow inputs. [Dependabot](../../.github/dependabot.yml) manages Go modules and GitHub Actions. Contributors must keep the Go pins in sync when reviewing updates.

## Proposed changes

### Install the repository toolchain

In each Go-consuming job, replace `actions/setup-go` with `jdx/mise-action`
after checkout. Pin the action to the full SHA of a verified stable release
with a version comment, following the repository's existing action convention.
Do not copy tool versions into workflow inputs such as `mise_toml` or
`tool_versions`: the checked-out `mise.toml` supplies every tool pin.

Use the action's `install: true`, `cache: true`, `env: true`, and
`export_path: true` behavior. Explicitly pin its `version` input to a reviewed
stable mise release. Let the action perform its repository trust/install step;
do not trust unrelated parent or child configs. Installation failures must fail the job.

Dependabot continues to update action SHAs, including `jdx/mise-action`.
Add a narrowly scoped Renovate regex manager for the action's `version` input,
annotated with datasource `github-releases` and dependency `jdx/mise`. Enable
`custom.regex` alongside the mise manager and include the migrated workflows in
Renovate's allowed paths. Use `ci` commits for mise bootstrap updates. The mise manager tracks tools in
`mise.toml`, and the Actions manager tracks action references, not this input.
Keep the bootstrap version consistent across migrated jobs and test it with
the existing tool pins before accepting an update.

Set `GOTOOLCHAIN: local` at workflow level as well as in mise. Log `mise --version`,
`mise ls`, and `mise exec -- go version` after installation. In Quality, assert
that the exact Go version in `mise.toml` matches the `go.mod` directive and
`mise exec -- go env GOVERSION`. A mismatch fails before compilation.

### Preserve the existing job boundaries

Keep the current job IDs, names, triggers, permissions, timeouts, build matrix,
and conventional-title/documentation/dependency checks.
Retain the `Required` aggregate and its dependency list. Do not add mise setup
to jobs that only invoke GitHub Actions, such as dependency review or the
aggregate gate. Preserve the Release Please and APT publishing workflows;
they do not compile Go.

Use `mise exec --` for direct tool commands and scripts that invoke Go. Preserve
the separate quality, builds, and vulnerability jobs instead of running
the entire local gate in every job:

| Job | Managed invocation |
| --- | --- |
| Quality | `mise exec -- gofmt -l .`, `mise exec -- go mod tidy`, `mise exec -- go mod verify`, `mise exec -- go vet ./...`, and `mise exec -- actionlint` |
| Builds | `GOOS=... GOARCH=... mise exec -- go build -buildvcs=false ./...` for the existing five targets |
| Vulnerabilities | Build the native binary with managed Go; run `mise exec -- govulncheck -mode=binary /tmp/motd-vulncheck` |

Keep module consistency checks that fail on diffs to `go.mod` or `go.sum`.
ShellCheck must be available to actionlint on PATH; new lint findings require
fixes or specific, explained suppressions. Do not disable ShellCheck globally.
Keep OS packaging utilities such as `apt-utils`, `dpkg-dev`, and GnuPG in the
publishing workflows that use them.

### Cache tools and Go builds separately

Use mise-action's default tool-cache key, which includes the mise version,
platform, and configuration hash. Avoid a custom key that reuses tools after a
pin change. Verify that an ARM runner restores ARM tools, not AMD64 binaries.

Replace setup-go's module/build cache with a SHA-pinned `actions/cache` step.
Cache the actual `mise exec -- go env GOMODCACHE` and `GOCACHE` paths; when a
Make target overrides them, cache those paths instead. Include runner OS,
architecture, and hashes of `mise.toml`, `go.mod`, and `go.sum` in the key.
An empty `go.sum` is valid for this stdlib-only project. Cache misses must still
install and run the exact pins. Never cache signing keys or generated releases.

### Release publishing and recovery

Install mise only after the immutable release tag is checked out and validated.
Build new releases with that tag's `mise.toml`; never load the config from
current `main`. Run packaging with `mise exec --` so nested Go commands inherit
the same compiler. Preserve signing, asset verification, upload permissions,
and the refusal to replace existing nonidentical assets.

Older tags have no mise config. Support manual recovery with an explicit
legacy branch: when the tag lacks `mise.toml`, read its exact `go.mod` directive,
install that Go version through mise, and invoke packaging with
`mise exec go@<tag-version> --`. Report the legacy selection. Never resolve
`latest` for an old tag or edit the tag to add a config. A tag that contains a
broken mise config must fail rather than entering the legacy branch. Verify
recovery against an actual pre-migration tag before changing the publisher.

## Validation

1. Run `mise install`, `mise run check`, and `mise run smoke` in a complete checkout. Confirm vet, all five cross-builds, binary vulnerability analysis, actionlint, and shell syntax checks pass. Record the built CLI's help and version output.
2. Validate the proposed Renovate configuration with its strict config validator. Check extraction for all four tools and every mise-action version input; verify Dependabot ownership stays distinct. A Go upgrade must change both the mise pin and `go.mod` in the same PR.
3. Run the CI workflow from a PR and a merge-group event. Confirm the same `CI / Required` context and the five release build targets. Fork PRs must work without privileged credentials or release secrets.
4. Exercise clean and warm caches on the runners used by the migrated jobs. Bump a tool pin on a branch and confirm the next run installs and reports that version. A stale cache must not select a different compiler.
5. Deliberately mismatch the two Go pins and verify a clear Quality failure. Test an invalid/unavailable tool pin and confirm setup fails rather than using runner-provided binaries.
6. Build and verify the complete release set with a disposable signing key. Test both a new tag with mise config and a pre-migration tag without it. Recovery must use each tag's exact compiler and retain byte-for-byte asset comparison.
7. Search workflows and Make targets for residual `actions/setup-go`, duplicated Go/tool versions, and `go run ...@v` installs. Account explicitly for any remaining occurrence before declaring migration complete.

## Delivery constraint

Implement any migration through a PR against protected `main`. Keep the `CI / Required` gate intact; do not change branch protection, create manual tags, or publish production releases to validate the proposal.
