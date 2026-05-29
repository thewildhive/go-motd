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
- `perf`: Performance improvements (creates a **minor** version)
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

Releases are automatically handled by the GitHub Actions release workflow on pushes to `main` (or manual dispatch):

1. **Quality gates**: `gofmt`, `go test`, `go vet`, and `go build` run first.
2. **Semantic versioning**: [`svu`](https://github.com/caarlos0/svu) analyses conventional commits since the last tag and determines the next version.
3. **Changelog**: Release notes are generated from git history between the last tag and HEAD.
4. **Tag and commit**: A `vX.Y.Z` tag is created on the release commit and pushed, and `CHANGELOG.md` is updated.
5. **Build binaries**: Cross-platform binaries are built with linker-injected `main.VERSION`.
6. **GitHub Release**: A release is created with archives, checksums, and changelog attached.

Releases run entirely with Go-native tooling — no npm or Node.js dependencies are involved.

## Development Workflow

1. Create a feature branch from `main`
2. Make your changes following the commit convention
3. Push your branch and create a pull request
4. Once merged to `main`, release automation evaluates commits and publishes a release only when commit types require one

## Testing

Before submitting a PR:

```bash
# Run tests
make test

# Build and run help smoke test
make smoke

# Check formatting
gofmt -l .

# Run static analysis
go vet ./...

# Build for multiple platforms
make cross-compile
```
