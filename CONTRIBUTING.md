# Contributing

## Submit a change

All changes, including releases, reach protected `main` through a pull request. Create a branch from `main`, push that branch, and open a PR against `main`. Do not push directly to `main` or bypass its required checks.

Use a [Conventional Commit](https://www.conventionalcommits.org/) PR title: squash merging makes it the commit subject that Release Please evaluates. For example, `fix(display): align resource labels` describes a patch, `feat(system): report reboot requests` describes a feature, and `docs: clarify installation` describes a documentation-only change. Use `!` or a `BREAKING CHANGE:` footer for an incompatible change. [Release Please configuration](release-please-config.json) defines the changelog sections.

Describe the impact, include evidence from the real CLI when behavior changes, and wait for `CI / Required` before squash merging. This gate aggregates quality, all five target builds, vulnerability scanning, and (on PRs) dependency review.

## Release Process

1. After a releasable change lands on `main`, [Release](.github/workflows/release.yml) uses the release GitHub App to open or update a PR containing the version and changelog. Do not hand-edit the version or create a tag to publish a release.
2. Review the release PR and merge it through the same protected path after `CI / Required` passes. Release Please then creates the `vX.Y.Z` tag and GitHub release.
3. [Publish Release](.github/workflows/publish-release.yml) builds from that tag and uploads five raw binaries, five archives, two Debian packages, and two checksum manifests with their Ed25519 signatures.
4. After publication succeeds, [Publish APT Repository](.github/workflows/apt-pages.yml) verifies the Debian packages, signs the repository metadata, and deploys to Pages. Confirm the release assets and APT deployment before announcing the release.

## Verifying a change

Install [mise](https://mise.jdx.dev/getting-started.html), then prepare the pinned
toolchain from the repository root:

```bash
mise trust mise.toml
mise install
```

`mise.toml` pins Go, govulncheck, actionlint, and ShellCheck. The Go version in
`go.mod` must match the mise pin. `GOTOOLCHAIN=local` prevents Go from silently
downloading a different compiler. Make targets run these tools through mise;
use `mise exec --` for direct commands such as `mise exec -- go vet ./...`.

Before submitting a PR, run the authoritative local gate:

```bash
mise run check
```

This covers formatting, module consistency, vet, all five release build targets, native vulnerability analysis, and workflow syntax. CI additionally validates internal Markdown links and dependency changes.

Run `make build` and exercise the changed behavior with `./bin/motd`. For terminal layout changes, capture before and after with the same configuration and terminal width; use `-no-color` if it makes the output easier to compare. Check `./bin/motd -no-config -json` when changing system presentation to confirm the JSON report is unchanged. Include commands and relevant output in the PR, with private values redacted. Do not add or run automated test suites.

## Toolchain Updates

[Renovate](renovate.json) monitors only the tools in `mise.toml`. It groups minor and patch updates, uses `chore(tools)` commits, and does not automerge. When changing Go, update `go.mod` and the mise pin together: local Make targets use mise, while CI and release publishing use `actions/setup-go` with `go.mod`. [Dependabot](.github/dependabot.yml) monitors Go modules and GitHub Actions. The [mise CI proposal](specs/mise-ci/TECH.md) describes a possible migration, not the current workflow.

## Release recovery and rollback

If asset publication fails, fix the cause and manually dispatch `Publish Release` with the unchanged `vX.Y.Z` tag. The workflow uploads missing assets, verifies identical existing assets, and fails rather than replacing a mismatch. Never move or recreate a published tag.

To roll back an installation, download the prior release's raw binary or archive, verify its signed checksum, and reinstall it. If a published release itself is defective, restore the prior version for users and issue a new patch release; do not mutate the defective release assets.
