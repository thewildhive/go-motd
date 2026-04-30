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
  -migrate        Migrate legacy config.yml/config.yaml to JSON and exit
  -no-config      Skip config loading and show system information only

Commands:
  self-update     Update to the latest version from GitHub releases
  self-update --force    Force update to latest version (even if current)
```

## Configuration

Configuration is JSON-only and optional. Without a config file, `motd` still displays system information and skips media integrations.

Supported config paths (priority order):
- `~/.config/motd/config.json`
- `/opt/motd/config.json`

Use `-config /path/to/config.json` to load a specific file, or `-no-config` to force system-only output.

Create a config file only when you want media integrations or custom system paths such as `tank_mount` or a fixed network interface.

If a legacy YAML config is detected (`config.yml`/`config.yaml`), `motd` exits with a migration message. Run `motd -migrate` to write the matching `config.json` next to the legacy file. With `-config /path/to/config.json`, migration looks for `/path/to/config.yml` or `/path/to/config.yaml` and writes the specified JSON path.

Legacy Organizr entries are not migrated because Organizr support was removed. The migrator skips them and reports the skipped service.

### Example Config

```json
{
  "services": {
    "plex": [
      {
        "name": "Main",
        "url": "http://plex:32400",
        "token": "your-plex-token",
        "enabled": true
      }
    ],
    "jellyfin": [
      {
        "name": "Main",
        "url": "http://jellyfin:8096",
        "token": "your-jellyfin-token",
        "enabled": true
      }
    ],
    "sonarr": [
      {
        "name": "Main",
        "url": "http://sonarr:8989",
        "api_key": "your-sonarr-api-key",
        "enabled": true
      }
    ],
    "radarr": [
      {
        "name": "Main",
        "url": "http://radarr:7878",
        "api_key": "your-radarr-api-key",
        "enabled": true
      }
    ],
    "seerr": [
      {
        "name": "Main",
        "url": "http://seerr:5055",
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

Use `config.json.sample` as the complete reference template. Media services are opt-in; each configured instance must be enabled and include both a URL and token/API key.

## System Information

`motd` displays core system information without config. Linux/macOS use standard Unix tools where available. Windows uses PowerShell/CIM first and falls back to WMIC/tasklist where possible.

Windows temperature and bandwidth can be unavailable on many systems because thermal sensors and `vnstat` are not consistently exposed by default.

## Seerr Integration

Pending request count is fetched directly from Seerr:
- Endpoint: `GET /api/v1/request/count`
- Header: `X-Api-Key: <seerr_api_key>`

## Self-Update

```bash
motd self-update
motd self-update --force
```

Security properties:
- SHA256 checksum verification
- HTTPS-only downloads
- Backup and rollback on update failure

Release uploads include raw binaries for `self-update`, human-friendly archives for manual installs, and separate checksum files for raw binaries and archives.

## Installation

See `INSTALL.md` for complete installation and system integration guidance.

## Development Checks

```bash
go test ./...
go vet ./...
gofmt -l .
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
