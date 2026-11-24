# MOTD - Go Implementation

A Go implementation of MOTD (Message of the Day) that displays system information and media service statistics with fast startup and native HTTP integrations.

Status: maintained. The latest release is the supported version; the CLI, JSON configuration, exit behavior, and published artifact names follow Semantic Versioning.

## Features

- Fast execution from a single compiled binary
- Built-in HTTP client with timeouts and connection reuse
- System information on Linux, macOS, and Windows with platform-specific fallbacks
- Optional multi-instance media service support (Plex, Jellyfin, Sonarr, Radarr, Seerr)
- Debian packages for amd64 and arm64; standalone builds retain signed self-update
- Cross-platform builds for Linux, macOS, and Windows

## Installation

- **Debian or Ubuntu (amd64/arm64):** [install `go-motd` with signed APT](INSTALL.md). The package installs `/usr/bin/motd`; use APT for updates.
- **Moving from a standalone binary to APT:** [migrate the existing installation](Migrate.md) so login hooks run the package binary. Your JSON config stays in place.
- **Standalone:** see the [Linux, macOS and Windows installation guide](docs/INSTALL.md#standalone-installation). These builds update with `motd self-update`.

## Build

From the repository root, use [mise](https://mise.jdx.dev/getting-started.html)
to install the pinned toolchain and build `bin/motd`:

```bash
mise trust mise.toml
mise install
mise run build
./bin/motd -no-config
```

## Usage

```text
motd [OPTIONS]

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
  self-update     Update a standalone installation from GitHub releases
  self-update --force    Install the latest standalone version without prompting
```

## Configuration

Configuration is JSON-only and optional. Without a config file, `motd` still displays system information and skips media integrations.

Supported config paths (priority order):
- `~/.config/motd/config.json`
- `/opt/motd/config.json`

Use `-config /path/to/config.json` to load a specific file, or `-no-config` to force system-only output. When `-config` is set, that exact JSON file must exist and parse successfully.

Create a config file only when you want media integrations, rootless container status, or custom system paths such as `tank_mount` or a fixed network interface. When `system.container_status` is configured, `motd` polls the local `motd-status-agent` Unix socket and shows a health-aware `Containers` summary. Missing or unavailable agent data is skipped silently.

When no default JSON config is found, a legacy `config.yml` or `config.yaml` in a default config directory causes an error. Automatic YAML migration ended in v2.0; see [Migrating to MOTD 2.0](MIGRATE_v2.md) for field mappings.

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
    "container_status": {
      "socket_path": "/var/run/motd-status/agent.sock",
      "max_age": "30s"
    },
    "tank_mount": "/mnt/tank",
    "network": {
      "interface": "eth0"
    }
  }
}
```

Use [config.json.sample](config.json.sample) as a complete reference. Enable only the instances you want to query (up to 32 per service type). Each enabled instance needs a URL and a token or API key. HTTPS is required for remote URLs; plaintext HTTP is accepted only for loopback hosts such as `localhost`, `127.0.0.1`, and `::1`. Run `motd check-config` to validate configuration; no config is valid.

## System Information

`motd` displays core system information without config. Linux/macOS use standard Unix tools where available. Windows uses PowerShell/CIM first and falls back to WMIC/tasklist where possible.

The terminal header uses the short hostname, with an optional `figlet` banner.
The `-json` report is a separate interface: it returns the version, configured
system paths/interface and optional reboot, container and media data. It does
not serialize the terminal's OS, uptime, memory and disk rows.

On Linux, `motd` shows a `Reboot` row with the value `Required` when `/run/reboot-required` (or its legacy location, `/var/run/reboot-required`) exists as a regular file. No root access is needed. With `--json`, the same signal appears as `system.reboot_required: true`; an omitted field does not mean no reboot is needed.

Windows temperature can be unavailable when firmware does not expose a compatible thermal sensor. Bandwidth reporting is not currently implemented on Windows.

### Trusted Directories for Optional Tools

Optional tools (vnstat, who, figlet, etc.) are resolved from a restricted set of trusted directories to prevent PATH hijacking in privileged contexts:

| Platform | Trusted Directories |
|----------|-------------------|
| Linux | `/usr/bin`, `/usr/sbin`, `/bin`, `/sbin` |
| macOS | `/usr/bin`, `/usr/sbin`, `/bin`, `/sbin`, `/usr/local/bin`, `/opt/homebrew/bin` |
| Windows | `C:\Windows\System32`, `C:\Windows\System32\WindowsPowerShell\v1.0` |

Install optional tools in the appropriate system directories using your package
manager. Do not link a user-writable program into a directory used by privileged
processes. Run `motd -d` to diagnose skipped services and unavailable data.

### Inspect container status

On a host with a running `motd-status-agent` and `system.container_status` enabled in your config, compare the terminal summary with the JSON workload details:

```bash
motd check-config
motd -no-color
motd -json
```

For a nonempty snapshot, the terminal shows the aggregate `Containers` line without workload names. JSON includes names and individual statuses. If the agent is unavailable, both forms omit container status rather than reporting zero workloads.

## Seerr Integration

Pending request count is fetched directly from Seerr:
- Endpoint: `GET /api/v1/request/count`
- Header: `X-Api-Key: <seerr_api_key>`

## Self-Update

For a standalone install in a directory you can write:

```bash
motd self-update
motd self-update --force
```

The updater verifies the Ed25519 signature on `checksums.txt` using its embedded
release key, then checks the downloaded binary's SHA-256. `--force` skips the
version check and confirmation, not the signature or checksum checks. Package
builds reject both commands; use [APT updates](INSTALL.md#update) instead. To
install a prior version, see [Roll back](docs/INSTALL.md#roll-back).

## Release Automation

Release Please opens a version and changelog PR from Conventional Commits on
`main`. Merging that PR creates a `vX.Y.Z` tag and GitHub Release. `Publish
Release` builds from that tag and signs two manifests: `checksums.txt` for the
five raw binaries, and `archive-checksums.txt` for their five archives and the
two Debian packages. A separate [APT publication workflow](docs/APT-REPOSITORY.md)
verifies the packages, signs the APT metadata and deploys to GitHub Pages.

To recover an incomplete release, dispatch `Publish Release` with the existing
tag. It uploads missing assets, accepts byte-identical files, and refuses to
overwrite different content. Publish a new patch rather than moving a tag or
replacing published assets.

## Development Checks

After [setting up the toolchain](#build), run the repository checks:

```bash
mise run check
```

`mise run build` builds `bin/motd`; `mise run smoke` builds it and prints its help and version.
Make also invokes toolchain commands through mise. See
[CONTRIBUTING.md](CONTRIBUTING.md#toolchain-updates) for Renovate setup and updates.

## Cross-Compilation

```bash
mise run cross-compile
mise exec -- make package VERSION=X.Y.Z SIGNING_KEY_FILE=/path/to/ed25519-private-key.pem
```

`cross-compile` writes raw binaries to `bin/` for Linux and macOS (amd64 and
arm64) and Windows (amd64). `package` rebuilds under `bin/release/` and requires
Linux packaging tools and an Ed25519 signing key; use a disposable key locally.
