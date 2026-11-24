# AGENTS.md

## Build Commands

Prepare the pins with `mise trust mise.toml` and `mise install`. Make targets
invoke managed tools; prefix direct toolchain commands with `mise exec --`.

- `make build` - Build regular binary to `bin/motd`
- `make build-optimized` - Build optimized binary with `-ldflags="-s -w"`
- `make smoke` - Build the optimized binary and run its help and version commands
- `make check` - Check formatting, module consistency, vet, native and cross-platform compilation, binary vulnerabilities, and workflows
- `make check-all` - Backward-compatible alias for `make check`
- `make clean` - Remove `bin/` directory
- `make cross-compile` - Build for all platforms
- `make package VERSION=X.Y.Z SIGNING_KEY_FILE=/path/to/key` - Build, sign, and verify the complete release asset set
- `go build -buildvcs=false -o bin/motd .` - Direct build
- `go vet ./...` - Run static analysis
- `gofmt -l .` - Check formatting

## Proving a Change

Do not add or run test suites. Build the CLI with `make build`, exercise the
changed path, and include the command and its relevant output in the PR. For
example, `./bin/motd -no-config -json` shows the system report without loading
service credentials. A change to a configured integration needs proof from
that integration, not just the help screen. Redact private values before sharing.

Use `make check` before submitting. It compiles all five release targets
because a native build skips files selected by other platforms' build tags.
Cross-compilation checks those files compile; it does not prove runtime behavior.

## GitHub Workflow

- Make future repository updates through pull requests instead of pushing directly to `main`.
- Use short-lived feature branches, open a PR against `main`, and monitor CI before merge.
- Wait for CI to pass on the PR before merging. If CI fails on a build-tag issue,
  the failed `Build` job identifies the target and compiler error.
- `Required` is the stable aggregate check context for the `main` ruleset
  (shown as `CI / Required` in the Actions UI).
- This is a solo-maintained repository; do not add CODEOWNERS or require human approvals.

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
| `perf` | patch | Code path/algorithm changes |
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
4. **Release trust boundary**: Release Please uses a short-lived GitHub App token.
   Publishing checks out the immutable release tag and never writes to `main`.
