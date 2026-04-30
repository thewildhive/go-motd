# AGENTS.md

## Build Commands

- `make build` - Build regular binary to bin/motd
- `make build-optimized` - Build optimized binary with -ldflags="-s -w"
- `make test` - Run `go test ./...`
- `make smoke` - Build optimized binary and show help
- `make clean` - Remove bin/ directory
- `make cross-compile` - Build for all platforms
- `go build -buildvcs=false -o bin/motd .` - Direct build
- `go vet ./...` - Run static analysis
- `gofmt -l .` - Check formatting
- `golangci-lint run` - Run comprehensive linting

## Code Style Guidelines

- Use standard Go formatting (`gofmt`)
- Import organization: stdlib, third-party, local (alphabetical within groups)
- Error handling: always check errors, use early returns
- Naming: PascalCase for exported, camelCase for unexported
- Constants: UPPER_SNAKE_CASE
- HTTP client: use global httpClient with 5s timeout and connection pooling
- Colors: use defined ANSI constants (RED, GREEN, YELLOW, BLUE, CYAN, BOLD, RESET)
- Functions: keep small, single responsibility, descriptive names
- No external dependencies beyond stdlib
