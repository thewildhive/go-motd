# Contributing

## Commit Message Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/) to enable automatic semantic versioning and changelog generation.

### Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

Only commit types that affect the compiled binary trigger a release:

- `feat`: New feature (creates a **minor** version)
- `fix`: Bug fix (creates a **patch** version)
- `perf`: Performance improvements (creates a **patch** version)
- `refactor`: Code restructuring (creates a **patch** version)
- `build`: Build system or dependency changes (creates a **patch** version)
- `BREAKING CHANGE:` in the footer creates a **major** version

Types that do **not** trigger a release (no effect on the compiled binary):

- `docs`: Documentation only
- `style`: Formatting, whitespace, code style only
- `test`: Adding or updating tests
- `ci`: CI/CD configuration changes
- `chore`: Maintenance tasks

### Examples

```bash
feat: add support for Jellyfin media server
fix: resolve memory leak in HTTP client
docs: update installation instructions
refactor: simplify configuration loading logic
test: add unit tests for bandwidth monitoring
```

## Release Process

Releases are managed by Release Please after reviewed pull requests are squash-merged to `main`:

1. Release Please opens or updates a reviewable release PR with the version and changelog.
2. Merging that release PR creates the immutable `vX.Y.Z` tag and GitHub Release.
3. A separate publishing workflow checks out that tag and uploads raw binaries, archives, checksums, and Ed25519 signatures.

Release Please uses a short-lived installation token from the portfolio release GitHub App. The publisher runs only for a published stable tag in this repository, or by an explicit manual recovery dispatch naming an existing tag. Existing assets are compared byte-for-byte and are never silently replaced.

## Development Workflow

1. Create a feature branch from `main`
2. Make your changes following the commit convention
3. Push your branch and create a pull request
4. Once merged to `main`, Release Please evaluates commits and opens or updates a release PR only when a release is needed

Automation never pushes directly to `main`; release metadata is merged through the same reviewed PR process.

## Testing

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

This covers formatting, module consistency, vet, deterministic and race tests, all five release build targets, native vulnerability analysis, and workflow syntax. CI additionally validates internal Markdown links and dependency changes.

## Toolchain Updates

`renovate.json` enables Renovate's built-in mise manager and the Go version
directive in `go.mod`. Go updates change both pins in one `build(toolchain)` PR;
other mise tool updates use `chore(tools)` and do not trigger releases. Updates
require review and passing checks; automerge is disabled. Dependabot continues
to manage Go module dependencies and GitHub Actions.

Enable the [Mend Renovate GitHub App](https://github.com/apps/renovate) for this
repository after merging the config, or include it in an existing self-hosted
Renovate installation. The config alone does not start a bot. Review the
Dependency Dashboard and the first update PR to confirm that all four tools
are detected and both Go pins change together.

The planned CI migration is documented in
[`specs/mise-ci/TECH.md`](specs/mise-ci/TECH.md).

## Release recovery and rollback

If asset publication fails, fix the cause and manually dispatch `Publish Release` with the unchanged `vX.Y.Z` tag. The workflow uploads missing assets, verifies identical existing assets, and fails rather than replacing a mismatch. Never move or recreate a published tag.

To roll back an installation, download the prior release's raw binary or archive, verify its signed checksum, and reinstall it. If a published release itself is defective, restore the prior version for users and issue a new patch release; do not mutate the defective release assets.
