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

Before submitting a PR, run the authoritative local gate:

```bash
make check
```

This covers formatting, module consistency, vet, deterministic and race tests, all five release build targets, native vulnerability analysis, and workflow syntax. CI additionally validates internal Markdown links and dependency changes.

## Release recovery and rollback

If asset publication fails, fix the cause and manually dispatch `Publish Release` with the unchanged `vX.Y.Z` tag. The workflow uploads missing assets, verifies identical existing assets, and fails rather than replacing a mismatch. Never move or recreate a published tag.

To roll back an installation, download the prior release's raw binary or archive, verify its signed checksum, and reinstall it. If a published release itself is defective, restore the prior version for users and issue a new patch release; do not mutate the defective release assets.
