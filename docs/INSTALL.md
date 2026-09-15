# Installation Guide

This guide covers installing and configuring `motd`.

## Debian and Ubuntu (v3+)

Releases include `go-motd_VERSION-1_amd64.deb` and `go-motd_VERSION-1_arm64.deb`.
These are statically linked, architecture-specific packages, not co-installable
Multi-Arch packages. The package installs `/usr/bin/motd`, a license, and a sample
config under `/usr/share/doc/go-motd/`. It does not change your config or login hooks.

Install v3.0.0, migrating the default standalone binary if present:

```bash
(
  set -eu
  installer=$(mktemp)
  trap 'rm -f "$installer"' EXIT
  curl -fLSs https://raw.githubusercontent.com/thewildhive/go-motd/v3.0.0/install-deb.sh -o "$installer"
  bash "$installer" 3.0.0 --migrate
)
hash -r
motd -v
```

Requires Bash, curl, OpenSSL with Ed25519 support, and apt/dpkg. Run as your normal
user; the helper uses sudo for installation. Review the downloaded script before
executing it if desired. It verifies the package against the release's signed
`archive-checksums.txt` using the embedded release public key before calling APT.

With `--migrate`, after a successful package installation the helper backs up the
old `/usr/local/bin/motd` to a uniquely named `motd.pre-deb.*` file, then replaces
the original with a symlink to `/usr/bin/motd`. Explicit login-hook paths keep
working. It refuses custom symlinks, unrecognized executables, and foreign
package-owned files. Omit `--migrate` to leave `/usr/local/bin` unchanged. Custom
install locations need manual migration; inspect `type -a motd` first.

For ongoing updates, configure the [APT repository](#apt-repository) once it is
activated. Without that source, `apt upgrade` cannot discover go-motd releases.
You can still repeat the helper with the desired version or install a verified
download with `sudo apt install ./go-motd_VERSION-1_ARCH.deb`.
Debian builds disable `self-update` (including `--force`) and GitHub update checks.
Standalone archives retain their existing updater.

Remove the package with `sudo apt remove go-motd`; user config remains intact.
If you migrated, also remove the compatibility symlink with
`sudo rm /usr/local/bin/motd` after checking it still points to `/usr/bin/motd`.
For rollback to the standalone install, remove the package and restore the
backup path printed by the helper to `/usr/local/bin/motd` using `sudo mv -T`.
Do not delete your config directories as part of a package migration or rollback.

## APT Repository

After the maintainer completes [Pages activation](APT-REPOSITORY.md), the default
repository URL is `https://thewildhive.github.io/go-motd/`. Do not run the setup
until the first deployment succeeds and the maintainer supplies the signing-key
fingerprint through a trusted channel. The repository hosts amd64 and arm64
packages in suite `stable`, component `main`; it does not replace Debian/Ubuntu
system repositories.

Run this in Bash, replacing the fingerprint placeholder with that trusted value.
Requires curl and GnuPG. It downloads the public key, verifies its fingerprint,
and installs a repository-scoped keyring rather than trusting it globally:

```bash
(
  set -eu
  expected='REPLACE_WITH_TRUSTED_40_CHARACTER_FINGERPRINT'
  base=https://thewildhive.github.io/go-motd
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  curl -fLSs "$base/go-motd-archive-keyring.gpg" -o "$work/key.gpg"
  actual=$(gpg --batch --show-keys --with-colons "$work/key.gpg" |
    awk -F: '$1 == "fpr" {print $10; exit}')
  test "$actual" = "$expected"
  sudo install -d -m 0755 /etc/apt/keyrings
  sudo install -m 0644 "$work/key.gpg" /etc/apt/keyrings/go-motd.gpg
  printf '%s\n' \
    'Types: deb' "URIs: $base/" 'Suites: stable' 'Components: main' \
    "Architectures: $(dpkg --print-architecture)" \
    'Signed-By: /etc/apt/keyrings/go-motd.gpg' > "$work/go-motd.sources"
  sudo install -m 0644 "$work/go-motd.sources" /etc/apt/sources.list.d/go-motd.sources
  sudo apt update
  sudo apt install go-motd
)
```

If v3.0.0 is already installed, no second migration is needed. Its old
`self-update` message predates repository support; continue using APT instead.
On later releases, use `sudo apt update && sudo apt install --only-upgrade go-motd`
or your normal `apt upgrade`. Automatic unattended upgrades require separately
allowing origin `go-motd` in unattended-upgrades settings.

Metadata expires after 30 days and is refreshed weekly. If APT reports expired
metadata, the maintainer must restore publication; do not disable signature or
expiry verification. During Pages/CDN propagation an index checksum mismatch can
temporarily occur: retry `apt update` later, without disabling verification.

To remove only the repository, delete `/etc/apt/sources.list.d/go-motd.sources`
and `/etc/apt/keyrings/go-motd.gpg`, then run `sudo apt update`. This leaves the
installed package and configuration intact.

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

Create a config only when you want media integrations or custom system paths. The wizard can write either the default location or an explicit path:

```bash
motd configure
motd configure -config /path/to/config.json
```

Media services are opt-in and each enabled instance must include a URL and token/API key. HTTPS is required for remote service URLs; plaintext HTTP is accepted only for loopback hosts such as `localhost`, `127.0.0.1`, and `::1`. Configure `system.container_status` to consume `motd-status-agent` over its local Unix socket; unavailable status is omitted from normal output and explained by `motd -d`.

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
- `motd-status-agent` — rootless Podman workload status, when configured
- `who` — logged-in user count

On Linux, most system information (memory, disk, uptime, CPU load, temperature, process count, network interface) is collected via `/proc` and system calls directly — no external tools required.

Windows system information uses PowerShell/CIM where possible and falls back to built-in commands such as `wmic` and `tasklist`. CPU temperature depends on sensor support; bandwidth reporting is not currently implemented on Windows.

Linux install example:

```bash
sudo apt install vnstat
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
