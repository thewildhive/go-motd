# Installation Guide

This guide covers installing and configuring `motd`.

## Quick Install

### Option 1: Download Pre-built Binary (Recommended)

1. Open the [Releases page](https://github.com/thewildhive/go-motd/releases)
2. Download the correct archive:
   - Linux: `motd-VERSION-linux-amd64.tar.gz` or `motd-VERSION-linux-arm64.tar.gz`
   - macOS: `motd-VERSION-darwin-amd64.tar.gz` or `motd-VERSION-darwin-arm64.tar.gz`
   - Windows: `motd-VERSION-windows-amd64.zip`
3. Extract and install:

```bash
# Linux/macOS
tar -xzf motd-*-linux-amd64.tar.gz
sudo mv motd-linux-amd64 /usr/local/bin/motd
sudo chmod +x /usr/local/bin/motd
```

```powershell
# Windows (PowerShell)
Expand-Archive motd-*-windows-amd64.zip
Move-Item motd-windows-amd64.exe C:\ProgramData\chocolatey\bin\motd.exe
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

`motd` now supports JSON config only.

Config lookup order:
1. `~/.config/motd/config.json`
2. `/opt/motd/config.json`

Legacy YAML files (`config.yml` / `config.yaml`) are not supported and will trigger a migration error.

Create config:

```bash
mkdir -p ~/.config/motd
cp config.json.sample ~/.config/motd/config.json
```

Then edit values for your environment.

### Services Supported

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
- `figlet`, `lolcat`
- `vnstat`
- `sensors`
- `docker`

Linux install example:

```bash
sudo apt install figlet lolcat vnstat lm-sensors docker.io
```

## Verify Installation

```bash
motd -v
motd -h
motd -d
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
