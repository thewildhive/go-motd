# Installation Guide

This guide covers installing and configuring `motd`.

## Quick Install

The commands below download the latest release binary for your platform, verify its SHA256 checksum, and install — one block to copy and paste. The raw binaries are listed in `checksums.txt` and are used directly (no archive extraction needed).

### Linux (amd64)
```bash
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/motd-linux-amd64"' | cut -d'"' -f4) -o motd-linux-amd64 &&\
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/checksums.txt"' | cut -d'"' -f4) -o checksums.txt &&\
sha256sum -c checksums.txt --ignore-missing &&\
sudo mv motd-linux-amd64 /usr/local/bin/motd &&\
sudo chmod +x /usr/local/bin/motd &&\
rm -f checksums.txt
```

### Linux (arm64)
```bash
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/motd-linux-arm64"' | cut -d'"' -f4) -o motd-linux-arm64 &&\
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/checksums.txt"' | cut -d'"' -f4) -o checksums.txt &&\
sha256sum -c checksums.txt --ignore-missing &&\
sudo mv motd-linux-arm64 /usr/local/bin/motd &&\
sudo chmod +x /usr/local/bin/motd &&\
rm -f checksums.txt
```

### macOS (Apple Silicon)
```bash
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/motd-darwin-arm64"' | cut -d'"' -f4) -o motd-darwin-arm64 &&\
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/checksums.txt"' | cut -d'"' -f4) -o checksums.txt &&\
shasum -a 256 -c checksums.txt --ignore-missing &&\
sudo mv motd-darwin-arm64 /usr/local/bin/motd &&\
sudo chmod +x /usr/local/bin/motd &&\
rm -f checksums.txt
```

### macOS (Intel)
```bash
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/motd-darwin-amd64"' | cut -d'"' -f4) -o motd-darwin-amd64 &&\
curl -sL $(curl -s https://api.github.com/repos/thewildhive/go-motd/releases/latest | grep browser_download_url | grep '/checksums.txt"' | cut -d'"' -f4) -o checksums.txt &&\
shasum -a 256 -c checksums.txt --ignore-missing &&\
sudo mv motd-darwin-amd64 /usr/local/bin/motd &&\
sudo chmod +x /usr/local/bin/motd &&\
rm -f checksums.txt
```

### Windows (PowerShell)
```powershell
$tag = (Invoke-RestMethod https://api.github.com/repos/thewildhive/go-motd/releases/latest).tag_name
$url = "https://github.com/thewildhive/go-motd/releases/download/${tag}/motd-windows-amd64.exe"
$csUrl = "https://github.com/thewildhive/go-motd/releases/download/${tag}/checksums.txt"
Invoke-WebRequest $url -OutFile motd-windows-amd64.exe
Invoke-WebRequest $csUrl -OutFile checksums.txt
$expected = (Get-Content checksums.txt | Where-Object { $_ -match 'motd-windows-amd64' } | ForEach-Object { ($_ -split '\s+')[0] })
$actual = (Get-FileHash motd-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
if ($expected -ne $actual) { throw "Checksum mismatch" }
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\motd" | Out-Null
Move-Item motd-windows-amd64.exe "$env:LOCALAPPDATA\Programs\motd\motd.exe" -Force
Remove-Item checksums.txt
```

### Build from Source

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

Use `motd -config /path/to/config.json` to load a specific file, or `motd -no-config` to force system-only output. When `-config` is set, that exact JSON file must exist and parse successfully. Use `motd check-config` to validate configuration; no config is valid and reports system-only mode.

Legacy YAML files (`config.yml` / `config.yaml`) are not loaded at runtime, and automatic YAML migration was removed in MOTD 2.0. See `MIGRATE_v2.md` for manual migration guidance. Legacy Organizr entries are unsupported because Organizr support was removed.

Create a config only when you want media integrations or custom system paths:

```bash
mkdir -p ~/.config/motd
cp config.json.sample ~/.config/motd/config.json
```

Then edit values for your environment. Media services are opt-in and each enabled instance must include a URL and token/API key. HTTPS is required for remote service URLs; plaintext HTTP is accepted only for loopback hosts such as `localhost`, `127.0.0.1`, and `::1`. If `compose_dir` points at directories with Compose files, `motd` shows a best-effort Docker Compose summary and skips it silently when unavailable.

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
motd --json -no-config
motd check-config
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
4. Run with `-d` for debug logs, including media services skipped because they are disabled, missing credentials, invalid URLs, or blocked remote HTTP

## Uninstall

```bash
sudo rm /usr/local/bin/motd
rm -rf ~/.config/motd
sudo rm -rf /opt/motd
```

## Roll back

Choose the prior known-good tag from the Releases page, replace `TAG` and `ASSET` below, and verify the raw binary against that release's signed `checksums.txt` before installing it. The normal installer and `self-update` command select only the latest release, so rollback is intentionally explicit.

```bash
TAG=v1.7.4
ASSET=motd-linux-amd64
curl -fL "https://github.com/thewildhive/go-motd/releases/download/${TAG}/${ASSET}" -o "${ASSET}"
curl -fL "https://github.com/thewildhive/go-motd/releases/download/${TAG}/checksums.txt" -o checksums.txt
curl -fL "https://github.com/thewildhive/go-motd/releases/download/${TAG}/checksums.txt.sig" -o checksums.txt.sig
# Verify checksums.txt.sig using the trusted Ed25519 public key, then:
sha256sum -c checksums.txt --ignore-missing
sudo install -m 0755 "${ASSET}" /usr/local/bin/motd
motd -v
```

Do not move an existing tag or replace its assets. Publish a new patch release for a corrected build.

## Help

- Issues: <https://github.com/thewildhive/go-motd/issues>
- Releases: <https://github.com/thewildhive/go-motd/releases>
