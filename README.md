# MOTD - Go Implementation

A Go implementation of MOTD (Message of the Day) that displays system information, service status, and media service statistics.

## Features

- **Fast Execution**: Compiled binary with quick startup time
- **Concurrent HTTP Requests**: Uses Go's native HTTP client with connection pooling
- **Low Memory Footprint**: Efficient memory management
- **Cross-Platform**: Can be compiled for different architectures

## Build

All build outputs are placed in the `bin/` directory:

```bash
# Build regular binary
make build
# Output: bin/motd

# Build optimized binary (recommended)
make build-optimized
# Output: bin/motd
```

Or using Go directly:

```bash
# Regular build
mkdir -p bin && go build -o bin/motd main.go

# Optimized build (smaller binary)
mkdir -p bin && go build -ldflags="-s -w" -o bin/motd main.go
```

The `-s -w` flags strip debug information and symbol tables, reducing binary size.

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

The application supports configuration via YAML configuration files.

### YAML Configuration

Create a YAML configuration file at one of these locations:
- `~/.config/motd/config.yml` (user config, highest priority)
- `/opt/motd/config.yml` (global config, fallback)

#### Example YAML Configuration

```yaml
services:
  plex:
    - name: "Main"
      url: "http://plex:32400"
      token: "your-plex-token"
      enabled: true
    - name: "Backup"
      url: "http://plex-backup:32400"
      token: "your-backup-token"
      enabled: true
  jellyfin:
    - name: "Main"
      url: "http://jellyfin:8096"
      token: "your-jellyfin-token"
      enabled: true
  sonarr:
    - name: "Main"
      url: "http://sonarr:8989"
      api_key: "your-sonarr-api-key"
      enabled: true
  radarr:
    - name: "Main"
      url: "http://radarr:7878"
      api_key: "your-radarr-api-key"
      enabled: true
  organizr:
    - name: "Main"
      url: "http://organizr:80"
      api_key: "your-organizr-api-key"
      enabled: true
system:
  compose_dir: "/opt/apps/compose"
  tank_mount: "/mnt/tank"
```

## Self-Update

The `motd` binary includes a self-update feature that allows you to update to the latest version directly from GitHub releases:

```bash
# Check for updates and install if available
motd self-update

# Force update to latest version (even if current)
motd self-update --force
```

### Security Features

- **Checksum Verification**: Downloads are verified using SHA256 checksums
- **HTTPS Only**: All downloads use secure HTTPS connections
- **Backup & Rollback**: Automatic backup creation with rollback on failure
- **Platform Detection**: Automatically detects your OS and architecture

### Update Process

1. Checks current version against latest GitHub release
2. Asks for confirmation before downloading (unless `--force` is used)
3. Downloads the appropriate binary for your platform
4. Verifies the binary integrity using checksums
5. Creates a backup of the current binary
6. Replaces the binary atomically
7. Cleans up temporary files

### Platform Support

Self-update supports the same platforms as the releases:
- Linux (amd64, arm64)
- macOS (amd64, arm64) 
- Windows (amd64)

## Technical Details

- **Startup time**: ~10-50ms typical
- **Memory usage**: ~5-10MB
- **HTTP Client**: Built-in HTTP client with connection pooling and timeout handling
- **Concurrency**: Can easily be extended to make API calls concurrently
- **Error Handling**: Robust error handling with graceful degradation
- **Type Safety**: Strongly typed data structures for API responses
- **Optional Dependencies**: `figlet`, `lolcat`, `sensors`, `vnstat`, `docker` (features work without them)

## Installation

### Quick Install

📖 **For detailed installation instructions, see [INSTALL.md](INSTALL.md)**

#### Option 1: Download Pre-built Binary (Recommended)

1. Go to the [Releases page](https://github.com/thewildhive/go-motd/releases)
2. Download the appropriate binary for your system
3. Extract and install to `/usr/local/bin/`

#### Option 2: Build from Source

```bash
git clone https://github.com/thewildhive/go-motd.git
cd go-motd
make install
```

### Manual Installation

1. Build the binary
2. Copy to `/usr/local/bin/` or another location in your PATH
3. Set up environment variables or YAML configuration
4. Run `motd` on login by adding to your shell profile

```bash
# Using Makefile
make install

# Or manually
sudo cp bin/motd /usr/local/bin/
sudo chmod +x /usr/local/bin/motd
```

## Automated Releases

This project uses **semantic-release** for automatic versioning and releases:

- **Automatic versioning** based on commit messages
- **Cross-platform binaries** generated for each release
- **Changelog** automatically generated
- **GitHub releases** created with all assets

### Commit Convention

Follow [Conventional Commits](https://www.conventionalcommits.org/) for automatic releases:
- `feat:` - New features (minor version)
- `fix:` - Bug fixes (patch version)
- `docs:` - Documentation changes (patch version)

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## Cross-Compilation

Build for different platforms (outputs to `bin/` directory):

```bash
# Build all platforms
make cross-compile

# Create release packages with version info
make package

# Full release process
make release

# Or manually for specific platforms:
mkdir -p bin

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/motd-linux-amd64 main.go

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/motd-linux-arm64 main.go

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/motd-darwin-amd64 main.go

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/motd-darwin-arm64 main.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/motd-windows-amd64.exe main.go
```
