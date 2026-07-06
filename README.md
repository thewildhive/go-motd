# MOTD - Go Implementation

A Go implementation of MOTD (Message of the Day) that displays system information and media service statistics with fast startup and native HTTP integrations.

## Features

- Fast execution from a single compiled binary
- Built-in HTTP client with timeouts and connection reuse
- System information on Linux, macOS, and Windows with platform-specific fallbacks
- Optional multi-instance media service support (Plex, Jellyfin, Sonarr, Radarr, Seerr)
- Self-update command with checksum verification
- Cross-platform builds for Linux, macOS, and Windows

## Build

All build outputs are placed in the `bin/` directory:

```bash
# Build regular binary
make build

# Build optimized binary
make build-optimized
```

Direct Go build:

```bash
mkdir -p bin && go build -buildvcs=false -o bin/motd .
mkdir -p bin && go build -buildvcs=false -ldflags="-s -w" -o bin/motd .
```

## Usage

```bash
./bin/motd [OPTIONS]

Options:
  -h              Show this help message
  -v              Show version information
  -d              Enable debug mode
  -config PATH    Load config from a specific JSON file
  -no-config      Skip config loading and show system information only
  -json           Output machine-readable JSON
  -no-color       Disable ANSI colors (also honors NO_COLOR)
  -services LIST  Only show selected media services (plex,jellyfin,sonarr,radarr,seerr)

Commands:
  configure       Create or edit the config file
  check-config    Validate configuration and print diagnostics
  self-update     Update to the latest version from GitHub releases
  self-update --force    Force update to latest version (even if current)
```

## Configuration

Configuration is JSON-only and optional. Without a config file, `motd` still displays system information and skips media integrations.

Supported config paths (priority order):
- `~/.config/motd/config.json`
- `/opt/motd/config.json`

Use `-config /path/to/config.json` to load a specific file, or `-no-config` to force system-only output. When `-config` is set, that exact JSON file must exist and parse successfully.

Create a config file only when you want media integrations or custom system paths such as `compose_dir`, `tank_mount`, or a fixed network interface. When `compose_dir` points at directories containing Compose files, `motd` shows a best-effort Docker Compose summary such as `All containers online` or `X of Y online`; missing or unavailable Compose data is skipped silently.

If a legacy YAML config is detected (`config.yml`/`config.yaml`), `motd` exits with an unsupported-config message. Automatic YAML migration was removed in MOTD 2.0; see `MIGRATE_v2.md` for manual guidance.

### Example Config

```json
{
  "services": {
    "plex": [
      {
        "name": "Main",
        "url": "https://plex.example.com:32400",
        "token": "your-plex-token",
        "enabled": true
      }
    ],
    "jellyfin": [
      {
        "name": "Main",
        "url": "https://jellyfin.example.com:8096",
        "token": "your-jellyfin-token",
        "enabled": true
      }
    ],
    "sonarr": [
      {
        "name": "Main",
        "url": "https://sonarr.example.com:8989",
        "api_key": "your-sonarr-api-key",
        "enabled": true
      }
    ],
    "radarr": [
      {
        "name": "Main",
        "url": "https://radarr.example.com:7878",
        "api_key": "your-radarr-api-key",
        "enabled": true
      }
    ],
    "seerr": [
      {
        "name": "Main",
        "url": "https://seerr.example.com:5055",
        "api_key": "your-seerr-api-key",
        "enabled": true
      }
    ]
  },
  "system": {
    "compose_dir": "/opt/apps/compose",
    "tank_mount": "/mnt/tank",
    "network": {
      "interface": "eth0"
    }
  }
}
```

Use `config.json.sample` as the complete reference template. Media services are opt-in; each configured instance must be enabled and include both a URL and token/API key. HTTPS is required for remote service URLs; plaintext HTTP is accepted only for loopback hosts such as `localhost`, `127.0.0.1`, and `::1`. Run `motd check-config` to validate configuration without treating a missing config as an error.

## System Information

`motd` displays core system information without config. Linux/macOS use standard Unix tools where available. Windows uses PowerShell/CIM first and falls back to WMIC/tasklist where possible.

Windows temperature and bandwidth can be unavailable on many systems because thermal sensors and `vnstat` are not consistently exposed by default.

### Trusted Directories for Optional Tools

Optional tools (docker, vnstat, who, figlet, etc.) are resolved from a restricted set of trusted directories to prevent PATH hijacking in privileged contexts:

| Platform | Trusted Directories |
|----------|-------------------|
| Linux | `/usr/bin`, `/usr/sbin`, `/bin`, `/sbin` |
| macOS | `/usr/bin`, `/usr/sbin`, `/bin`, `/sbin`, `/usr/local/bin`, `/opt/homebrew/bin` |
| Windows | `C:\Windows\System32`, `C:\Windows\System32\WindowsPowerShell\v1.0` |

If you install optional tools in non-standard paths (e.g., `/snap/bin/docker`, `~/bin/figlet`), create a symlink from a trusted directory:

```bash
sudo ln -s /snap/bin/docker /usr/bin/docker
```

Run with `-d` (debug) to see which tools are not found and why configured media services are skipped.

## Seerr Integration

Pending request count is fetched directly from Seerr:
- Endpoint: `GET /api/v1/request/count`
- Header: `X-Api-Key: <seerr_api_key>`

## Self-Update

```bash
motd self-update
motd self-update --force
```

### Security Model

The self-update mechanism provides defense-in-depth against supply-chain attacks:

| Layer | Protection | Breaks If |
|-------|-----------|-----------|
| HTTPS | Transport encryption; prevents MITM during download | CA compromise, TLS downgrade |
| SHA-256 checksums | Detects corruption or tampering of the downloaded binary | Checksum file is also tampered (no origin verification) |
| Ed25519 signature | Proves `checksums.txt` was published by the maintainer | Private signing key is compromised |
| Trusted PATH | Prevents `motd self-update` from launching untrusted helper binaries | System trusted directories are writable by attacker |

Operational assumptions:
- The signing private key is stored as a GitHub repository secret (`SIGNING_PRIVATE_KEY`)
- The corresponding public key is compiled into the binary (`checksumsPublicKeyHex`)
- Releases are created by the CI pipeline; assets are signed during release automation
- `motd self-update` requires write access to the binary's directory; use `sudo` or install to `~/.local/bin` if needed
- Cross-device binary replacement uses a staged copy in the target directory before atomic rename

### Windows Notes

The Windows updater writes a batch script to a random path in `%TEMP%` that schedules replacement after the current process exits. Metacharacters in file paths (`%`, `^`, `&`, `|`, `<`, `>`, `()`) are escaped for batch safety.

## Installation

See `docs/INSTALL.md` for complete installation and system integration guidance.

## Release Automation

Releases are generated from conventional commits on `main` using Go-native tooling — [`svu`](https://github.com/caarlos0/svu) for version bumping and a shell-based pipeline for build and publish. No npm/Node.js dependencies are involved.

- `feat` and `perf` trigger minor releases
- `fix`, `refactor`, and `build` trigger patch releases
- `docs`, `style`, `test`, `ci`, and `chore` do not trigger releases (they don't affect the compiled binary)
- `BREAKING CHANGE:` in commit footer triggers a major release

When a release is created, CI builds cross-platform archives and uploads them with `checksums.txt` to the GitHub release for the new `vX.Y.Z` tag. The release workflow may commit the generated `CHANGELOG.md` update directly to `main`; that automation commit is the explicit exception to the pull-request-only workflow.

## Development Checks

```bash
go test ./...
go vet ./...
gofmt -l .
make check-all
make cross-compile
```

## Cross-Compilation

```bash
make cross-compile
make package
```

Manual examples:

```bash
VERSION=dev
GOOS=linux GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w -X main.VERSION=${VERSION}" -o bin/motd-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -buildvcs=false -ldflags="-s -w -X main.VERSION=${VERSION}" -o bin/motd-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w -X main.VERSION=${VERSION}" -o bin/motd-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -buildvcs=false -ldflags="-s -w -X main.VERSION=${VERSION}" -o bin/motd-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w -X main.VERSION=${VERSION}" -o bin/motd-windows-amd64.exe .
```
