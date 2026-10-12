# Use mise in CI

Status: proposed. This change prepares local mise pins and Renovate; workflow
migration is a separate implementation task.

## Context

The goal is to run local development, pull-request checks, and release builds
with the same reviewed tool versions. `mise.toml` now pins Go, govulncheck,
actionlint, and ShellCheck. The Go directive in `go.mod` matches the compiler
pin, and `GOTOOLCHAIN=local` prevents implicit toolchain downloads. Make targets
invoke the managed executables instead of installing tools with `go run ...@v`.

Preparation started in a snapshot without usable Git metadata or `.github`.
Workflow research and subsequent merge validation use a complete checkout of
public `main` at commit `c97d867bc2b7b37e9a1be118785ff0193ba8a5fa`. The checkout
includes Debian packaging checks missing from the initial snapshot; those
checks are preserved. Re-read the actual branch before CI implementation.
Relevant baseline files:

- [`ci.yml:25`](https://github.com/thewildhive/go-motd/blob/c97d867bc2b7b37e9a1be118785ff0193ba8a5fa/.github/workflows/ci.yml#L25): `quality`, `tests`, `builds`, `debian`, and `vulnerabilities` use `actions/setup-go` with `go-version-file: go.mod`. Quality pins actionlint separately; vulnerabilities pins govulncheck separately.
- [`ci.yml:173`](https://github.com/thewildhive/go-motd/blob/c97d867bc2b7b37e9a1be118785ff0193ba8a5fa/.github/workflows/ci.yml#L173): `Required` aggregates the checks and is the stable ruleset context.
- [`publish-release.yml:42`](https://github.com/thewildhive/go-motd/blob/c97d867bc2b7b37e9a1be118785ff0193ba8a5fa/.github/workflows/publish-release.yml#L42): the publisher checks out an immutable release tag before setting up Go and packaging assets.
- [`package-release.sh:35`](https://github.com/thewildhive/go-motd/blob/c97d867bc2b7b37e9a1be118785ff0193ba8a5fa/.github/scripts/package-release.sh#L35): packaging invokes Go directly for five targets, then Debian packages and signed archives.
- [`dependabot.yml:3`](https://github.com/thewildhive/go-motd/blob/c97d867bc2b7b37e9a1be118785ff0193ba8a5fa/.github/dependabot.yml#L3): Dependabot owns Go module dependencies and GitHub Actions updates.

## Proposed changes

### Install the repository toolchain

In each Go-consuming job, replace `actions/setup-go` with `jdx/mise-action`
after checkout. Pin the action to the full SHA of a verified stable release
with a version comment, following the repository's existing action convention.
Do not copy tool versions into workflow inputs such as `mise_toml` or
`tool_versions`: the checked-out `mise.toml` supplies every tool pin.

Use the action's `install: true`, `cache: true`, `env: true`, and
`export_path: true` behavior. Explicitly pin its `version` input to a reviewed
stable mise release. Start with the verified local release `2026.10.8` or a newer
stable version resolved and tested at implementation time. Let the action
perform its repository trust/install step; do not trust unrelated parent or
child configs. Installation failures must fail the job.

Dependabot continues to update action SHAs, including `jdx/mise-action`.
Add a narrowly scoped Renovate regex manager for the action's `version` input,
annotated with datasource `github-releases` and dependency `jdx/mise`. Enable
`custom.regex` alongside the current managers and use `ci` commits for mise
bootstrap updates. This is needed because the mise manager tracks tools in
`mise.toml`, and the Actions manager tracks action references, not this input.
Keep the bootstrap version consistent across migrated jobs and test it with
the existing tool pins before accepting an update.

Set `GOTOOLCHAIN: local` at workflow level as well as in mise. Log `mise --version`,
`mise ls`, and `mise exec -- go version` after installation. In Quality, assert
that the exact Go version in `mise.toml` matches the `go.mod` directive and
`mise exec -- go env GOVERSION`. A mismatch fails before builds or tests run.

### Preserve the existing job boundaries

Keep the current job IDs, names, triggers, permissions, timeouts, matrices,
coverage output, and conventional-title/documentation/dependency checks.
Retain the `Required` aggregate and its dependency list. Do not add mise setup
to jobs that only invoke GitHub Actions, such as dependency review or the
aggregate gate. Preserve the Release Please and APT publishing workflows;
they do not compile Go in the researched baseline.

Use `mise exec --` for direct tool commands and scripts that invoke Go. Preserve
the separate quality, tests, builds, and vulnerability jobs instead of running
the entire local gate in every job:

| Job | Managed invocation |
| --- | --- |
| Quality | `mise exec -- gofmt -l .`, `mise exec -- go mod tidy`, `mise exec -- go mod verify`, `mise exec -- go vet ./...`, and `mise exec -- actionlint` |
| Tests | `mise exec -- go test -race -count=1 -covermode=atomic -coverprofile=coverage.out ./...` and `mise exec -- go tool cover -func=coverage.out` |
| Builds | `GOOS=... GOARCH=... mise exec -- go build -buildvcs=false ./...` for the existing five targets |
| Debian | `mise exec -- bash .github/scripts/test-deb.sh` and the existing installer/APT test scripts |
| Vulnerabilities | Build the native binary with managed Go; run `mise exec -- govulncheck -mode=binary /tmp/motd-vulncheck` |

Keep module consistency checks that fail on diffs to `go.mod` or `go.sum`.
ShellCheck must be available to actionlint on PATH; new lint findings require
fixes or specific, explained suppressions. Do not disable ShellCheck globally.
Keep OS packaging utilities such as `apt-utils`, `dpkg-dev`, and GnuPG in their
existing system-package installation steps.

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

## Testing and validation

1. Run `mise install`, `mise run check`, and `mise run smoke` in a complete checkout. Keep native vet/tests/race tests, all five cross-builds, binary vulnerability analysis, actionlint, and shell syntax checks passing.
2. Validate Renovate with its official strict config validator. Inspect extraction and a first update PR: Go changes both pins together; validation tools are detected; the new bootstrap manager finds every mise-action version input; Dependabot ownership stays distinct.
3. Run the CI workflow from a PR and a merge-group event. Confirm the same `CI / Required` context and applicable checks, including AMD64/ARM Debian jobs. Fork PRs must work without privileged credentials or release secrets.
4. Exercise clean and warm caches on both runner architectures. Bump a tool pin in a test branch and confirm the next run installs and reports that version. A stale cache must not select a different compiler.
5. Deliberately mismatch the two Go pins and verify a clear Quality failure. Test an invalid/unavailable tool pin and confirm setup fails rather than using runner-provided binaries.
6. Build and verify the complete release set with a disposable signing key. Test both a new tag with mise config and a pre-migration tag without it. Recovery must use each tag's exact compiler and retain byte-for-byte asset comparison.
7. Search workflows and Make targets for residual `actions/setup-go`, duplicated Go/tool versions, and `go run ...@v` installs. Account explicitly for any remaining occurrence before declaring migration complete.

Preparation verified on Linux ARM64 with Go `1.27.2`: formatting, module
consistency, vet, tests, race tests, all five cross-builds, native vulnerability
analysis, workflow validation, and CLI smoke tests passed. Debian package
install, upgrade, removal, config preservation, and updater checks passed in a
disposable dpkg root outside the filesystem sandbox. Renovate `44.150.1` passed
strict config validation and extraction/rule checks for all four tools and
both grouped Go pins.

The existing Renovate onboarding PR is connected to the repository. Before
the Go upgrade, its vulnerability check reported eight standard-library
findings fixed in `1.27.2`. Live Actions validation of the proposed CI migration,
cache behavior, and historical-tag release recovery remains required during
implementation.

## Parallelization

Use one implementation branch, `ci/use-mise`, and one PR. CI setup, caches,
Renovate bootstrap matching, and release recovery share toolchain assumptions;
sequential implementation and validation are simpler than assigning file
ownership to parallel agents. Implement CI first, then publisher recovery, then
run the full gate and review the first Renovate update.
