# Installation Guide

This guide covers installing and configuring `motd`.

## Quick Install

### Option 1: Download Pre-built Binary (Recommended)

1. Open the [Releases page](https://github.com/thewildhive/go-motd/releases)
2. Download the correct archive and `archive-checksums.txt`:
   - Linux: `motd-VERSION-linux-amd64.tar.gz` or `motd-VERSION-linux-arm64.tar.gz`
   - macOS: `motd-VERSION-darwin-amd64.tar.gz` or `motd-VERSION-darwin-arm64.tar.gz`
   - Windows: `motd-VERSION-windows-amd64.zip`
3. Verify the archive checksum, then extract and install:

```bash
# Linux amd64
sha256sum -c archive-checksums.txt --ignore-missing
tar -xzf motd-*-linux-amd64.tar.gz
sudo mv motd-linux-amd64 /usr/local/bin/motd
sudo chmod +x /usr/local/bin/motd
```

```bash
# macOS Apple Silicon
shasum -a 256 -c archive-checksums.txt --ignore-missing
tar -xzf motd-*-darwin-arm64.tar.gz
sudo mv motd-darwin-arm64 /usr/local/bin/motd
sudo chmod +x /usr/local/bin/motd
```

```powershell
# Windows (PowerShell)
Expand-Archive motd-*-windows-amd64.zip
Set-Location motd-*-windows-amd64
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\motd" | Out-Null
Move-Item motd-windows-amd64.exe "$env:LOCALAPPDATA\Programs\motd\motd.exe"
```

### Option 2: Build from Source

```bash
git clone https://github.com/thewildhive/go-motd.git
cd go-motd
make build-optimized
sudo cp bin/motd /usr/local/bin/motd
sudo chmod +x /usr/local/bin/motd
```

## Configuration

`motd` supports JSON config only. The config file is optional; without it, `motd` displays system information and skips media integrations.

Config lookup order:
1. `~/.config/motd/config.json`
2. `/opt/motd/config.json`

Use `motd -config /path/to/config.json` to load a specific file, or `motd -no-config` to force system-only output.

Legacy YAML files (`config.yml` / `config.yaml`) are not loaded at runtime. To migrate one to JSON, run:

```bash
motd -migrate
motd -config /path/to/config.json -migrate
```

The migrator writes the matching JSON path and refuses to overwrite an existing JSON config. Legacy Organizr entries are skipped because Organizr support was removed.

Create a config only when you want media integrations or custom system paths:

```bash
mkdir -p ~/.config/motd
cp config.json.sample ~/.config/motd/config.json
```

Then edit values for your environment. Media services are opt-in and each enabled instance must include a URL and token/API key.

### Optional Media Services

- Plex (`token`)
- Jellyfin (`token`)
- Sonarr (`api_key`)
- Radarr (`api_key`)
- Seerr (`api_key`)

Seerr pending requests are read from:
- `GET /api/v1/request/count`
- `X-Api-Key` header

## Optional Runtime Tools

Optional commands used for richer output:
- `vnstat` — monthly bandwidth estimates (falls back gracefully if absent)
- `docker` — container count
- `who` — logged-in user count

Most system information (memory, disk, uptime, CPU load, temperature, process count, network interface) is collected via `/proc` and `syscall` directly — no external tools required.

Windows system information uses PowerShell/CIM where possible and falls back to built-in commands such as `wmic` and `tasklist`. CPU temperature and bandwidth may be unavailable on Windows depending on sensor/collector support.

Linux install example:

```bash
sudo apt install vnstat docker.io
```

## Verify Installation

```bash
motd -v
motd -h
motd -d
motd -no-config
```

## Shell Integration

Add `motd` to your shell profile:

```bash
echo 'motd' >> ~/.bashrc
```

For zsh:

```bash
echo 'motd' >> ~/.zshrc
```

## Troubleshooting

### Command Not Found

```bash
which motd
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
```

### Config Issues

Missing config is valid and should still produce system information. If you expect media output, verify the JSON config exists and has enabled services:

```bash
ls -la ~/.config/motd/config.json
ls -la /opt/motd/config.json
motd -d
```

### Service/API Issues

1. Verify service URL is reachable
2. Verify API keys/tokens
3. Verify firewall/network access
4. Run with `-d` for debug logs

## Uninstall

```bash
sudo rm /usr/local/bin/motd
rm -rf ~/.config/motd
sudo rm -rf /opt/motd
```

## Help

- Issues: <https://github.com/thewildhive/go-motd/issues>
- Releases: <https://github.com/thewildhive/go-motd/releases>
