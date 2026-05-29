# AGENTS.md

## Build Commands

- `make build` - Build regular binary to `bin/motd`
- `make build-optimized` - Build optimized binary with `-ldflags="-s -w"`
- `make test` - Run `go test ./...`
- `make smoke` - Build optimized binary and show help
- `make check` - Run formatting check, vet, and tests
- `make clean` - Remove `bin/` directory
- `make cross-compile` - Build for all platforms
- `make svu-version` - Show current and next semantic versions via `svu`
- `go build -buildvcs=false -o bin/motd .` - Direct build
- `go vet ./...` - Run static analysis
- `gofmt -l .` - Check formatting

## Critical: Multiplatform Verification Before Any Commit

Many source files have platform build tags (`//go:build windows`, `//go:build darwin`,
`//go:build !windows && !darwin`). On a Linux development machine, `go build`, `go vet`,
and `go test` **skip these files entirely**. A missing import or undefined symbol in a
platform-tagged file will not be caught locally — it will surface only in CI or the
Release pipeline.

**Before committing any code change, run all of these:**

```bash
# Native (linux/amd64) — catches most issues
go build ./...
go vet ./...
go test ./... -count=1

# Cross-platform compilation — catches build-tag-hidden errors
GOOS=windows GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
```

The CI pipeline does this automatically via the "Cross-platform compilation check"
step, but do not rely on CI alone — the earlier you catch a platform mismatch,
the less rework is needed.

## GitHub Workflow

- Make future repository updates through pull requests instead of pushing directly to `main`.
- Use short-lived feature branches, open a PR against `main`, and monitor CI before merge.
- Wait for CI to pass on the PR before merging. If CI fails on a build-tag issue,
  the Cross-platform compilation check log will show which target and symbol failed.

## Code Style Guidelines

- Use standard Go formatting (`gofmt`)
- Import organization: stdlib, third-party, local (alphabetical within groups)
- Error handling: always check errors, use early returns
- Naming: PascalCase for exported, camelCase for unexported
- Constants: UPPER_SNAKE_CASE
- HTTP client: create a local `*http.Client` with 5s timeout — avoid globals
- Colors: use `display.Red`, `display.Green`, `display.Blue`, `display.Cyan`, `display.Bold`, `display.Reset`
- Functions: keep small, single responsibility, descriptive names
- No external dependencies beyond stdlib

## Package Layout

```
motd/
├── main.go              # CLI entry, flag parsing, subcommand routing
├── configure.go         # Interactive setup wizard (motd configure)
├── migration.go         # Legacy YAML → JSON config migration
├── config/              # Config struct, loading, saving, paths
├── display/             # ANSI colors, DotLabel, PrintSection, PrintHeader, DebugLog
├── media/               # Plex, Jellyfin, Sonarr, Radarr, Seerr clients
├── system/              # OS info, disk, uptime, memory, bandwidth, temp, processes
│   ├── system.go        # Shared helpers, ConfigAccessor, FormatDuration, GetDefaultInterface
│   ├── unix.go          # Linux implementations (build tag: !windows,!darwin)
│   ├── darwin.go        # macOS implementations (build tag: darwin)
│   └── windows.go       # Windows implementations (build tag: windows)
├── update/              # Self-update with checksum verification
└── util/                # GetUserHome, HasCommand, CopyFile, PluralSuffix
```

## Release Rules

Only commit types that produce a different compiled binary trigger a release:

| Type | Bump | Rationale |
|------|------|-----------|
| `feat` | minor | New code compiled in |
| `fix` | patch | Bugfix changes the binary |
| `perf` | minor | Code path/algorithm changes |
| `refactor` | patch | Code restructuring alters the artifact |
| `build` | patch | Dependency/compiler flag changes |
| `docs` | — | Only .md files, no Go source changes |
| `style` | — | Formatting only, binary is semantically identical |
| `test` | — | Test files not compiled into release binary |
| `ci` | — | Workflow/config only |
| `chore` | — | Maintenance, no source changes |
| BREAKING | major | API contract changes |

## Common Pitfalls

1. **Build-tag blindness**: A function added to `system/unix.go` must also have a
   stub in `system/darwin.go` and `system/windows.go` if called from `system/system.go`.
   Always run cross-platform builds before committing.
2. **Global state regression**: Do not reintroduce package-level globals for
   `config`, `httpClient`, or `debugMode`. Pass them explicitly.
3. **Import sorting**: `gofmt` handles this. Run `gofmt -l .` and fix any
   unformatted files — the CI gate will reject them.
4. **Concurrent merge races**: The Release workflow uses `cancel-in-progress: true`
   and pulls latest `main` before pushing. If merging multiple PRs quickly, let
   each release run complete before merging the next one.
