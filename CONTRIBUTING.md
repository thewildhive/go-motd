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

- `feat`: New feature (creates a **minor** version)
- `fix`: Bug fix (creates a **patch** version)
- `docs`: Documentation changes (**no release** by default)
- `style`: Code style changes (formatting, etc.) (creates a **patch** version)
- `refactor`: Code refactoring (creates a **patch** version)
- `perf`: Performance improvements (creates a **minor** version)
- `test`: Adding or updating tests (creates a **patch** version)
- `build`: Build system or dependency changes (creates a **patch** version)
- `ci`: CI/CD configuration changes (**no release** by default)
- `chore`: Maintenance tasks (**no release** by default)

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
2. **Semantic versioning**: `semantic-release` analyzes Conventional Commit messages and decides whether to publish.
3. **Tag and changelog**: On publish, `semantic-release` creates the `v<version>` tag, updates `CHANGELOG.md`, and pushes a `chore(release)` commit.
4. **GitHub release**: `semantic-release` publishes release notes and creates the GitHub release entry.
5. **Release assets**: The workflow builds cross-platform binaries with `-X main.VERSION=<version>`, packages archives, generates `checksums.txt`, and uploads assets to the same tag.

## Development Workflow

1. Create a feature branch from `main`
2. Make your changes following the commit convention
3. Push your branch and create a pull request
4. Once merged to `main`, release-worthy commits will automatically publish a release

Commit messages should follow the documented types so semantic-release can compute the correct version bump.

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
