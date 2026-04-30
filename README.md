# MOTD - Go Implementation

A Go implementation of MOTD (Message of the Day) that displays system information and media service statistics with fast startup and native HTTP integrations.

## Features

- Fast execution from a single compiled binary
- Built-in HTTP client with timeouts and connection reuse
- Multi-instance media service support (Plex, Jellyfin, Sonarr, Radarr, Seerr)
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
mkdir -p bin && go build -o bin/motd main.go
mkdir -p bin && go build -ldflags="-s -w" -o bin/motd main.go
```

## Usage

```bash
./motd [OPTIONS]

Options:
  -h              Show this help message
  -v              Show version information
  -d              Enable debug mode

Commands:
  self-update     Update to the latest version from GitHub releases
  self-update --force    Force update to latest version (even if current)
```

## Configuration

Configuration is JSON-only.

Supported config paths (priority order):
- `~/.config/motd/config.json`
- `/opt/motd/config.json`

If a legacy YAML config is detected (`config.yml`/`config.yaml`), `motd` exits with a migration message.

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

Use `config.json.sample` as the complete reference template.

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
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/motd-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/motd-linux-arm64 main.go
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/motd-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/motd-darwin-arm64 main.go
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/motd-windows-amd64.exe main.go
```
