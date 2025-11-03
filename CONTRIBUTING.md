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
- `docs`: Documentation changes (creates a **patch** version)
- `style`: Code style changes (formatting, etc.) (creates a **patch** version)
- `refactor`: Code refactoring (creates a **patch** version)
- `perf`: Performance improvements (creates a **minor** version)
- `test`: Adding or updating tests (creates a **patch** version)
- `build`: Build system or dependency changes (creates a **patch** version)
- `ci`: CI/CD configuration changes (creates a **patch** version)
- `chore`: Maintenance tasks (creates a **patch** version)

### Examples

```bash
feat: add support for Jellyfin media server
fix: resolve memory leak in HTTP client
docs: update installation instructions
refactor: simplify configuration loading logic
test: add unit tests for bandwidth monitoring
```

## Release Process

Releases are automatically created when commits are pushed to the `main` branch:

1. **Analyze commits**: Semantic-release analyzes commit messages
2. **Determine version**: Based on commit types (feat=minor, fix=patch, etc.)
3. **Create tag**: Git tag is created with new version
4. **Generate changelog**: Automatic changelog is generated
5. **Create release**: GitHub release is created with assets
6. **Build binaries**: Cross-platform binaries are built and attached

## Development Workflow

1. Create a feature branch from `main`
2. Make your changes following the commit convention
3. Push your branch and create a pull request
4. Once merged to `main`, a release will be automatically created

## Testing

Before submitting a PR:

```bash
# Run tests
make test

# Check formatting
gofmt -l .

# Run static analysis
go vet ./...

# Build for multiple platforms
make cross-compile
```