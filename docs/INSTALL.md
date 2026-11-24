# Installation Guide

## Debian and Ubuntu (v3+)

On amd64 and arm64, follow the [signed APT installation guide](../INSTALL.md).
If you already have a standalone binary, follow [Migrate to Debian packages](../Migrate.md)
instead so your shell and login hooks use `/usr/bin/motd`. The `go-motd` package
installs a sample in `/usr/share/doc/go-motd/`, not a live config or login hook.
Package builds skip GitHub update checks and reject `motd self-update`.

### APT repository

Use [the client setup](../INSTALL.md#install) for the repository URL, trusted
fingerprint and scoped keyring. If you installed a `.deb` directly (including
with the v3.0.0 migration helper), add the source once to get updates from APT.

- **Key or signature error:** stop. Check the fingerprint against the client
  setup. Do not trust a new key merely because APT reports a signature error.
- **Expired metadata:** check the system clock. If it is correct, the maintainer
  must [refresh the signed repository](APT-REPOSITORY.md#publication-and-recovery).
  Metadata expires after 30 days and normally refreshes weekly.
- **Index hash mismatch:** retry `sudo apt update` after Pages/CDN propagation.
  Do not disable signature, checksum or expiry checks.

## Standalone installation

### Linux and macOS (amd64 and arm64)

Use the [standalone installer](../install.sh) from a trusted checkout. It selects
the **latest** release for your host, verifies the Ed25519 signature on
`archive-checksums.txt` and checks the archive's SHA-256 before extracting it.
You need Bash, tar, curl or wget, and OpenSSL with Ed25519 `pkeyutl -rawin`
support. Do not run it over a package-managed installation.

For a per-user install, run these commands from the repository root:

```bash
mkdir -p "$HOME/.local/bin" "$HOME/.config/motd"
bash install.sh --dir "$HOME/.local/bin"
"$HOME/.local/bin/motd" -v
```

Add `$HOME/.local/bin` to your `PATH` to use the command as `motd`. Without
`--dir`, the installer targets `/usr/local/bin` and may request sudo. It creates
a sample config in `~/.config/motd` if missing; review the example paths before use.

### Windows (amd64)

In an empty working directory, download `motd-windows-amd64.exe`, `checksums.txt`
and `checksums.txt.sig` from the **same** [GitHub release](https://github.com/thewildhive/go-motd/releases).
Save the `BEGIN PUBLIC KEY` through `END PUBLIC KEY` lines from a trusted copy of
[install-deb.sh](../install-deb.sh) as ASCII `release-public.pem`. This is the
Ed25519 **release** key, not the APT OpenPGP key. With OpenSSL supporting Ed25519
available in PowerShell, verify the signature and the exact binary before installation:

```powershell
$ErrorActionPreference = 'Stop'
openssl pkeyutl -verify -pubin -inkey release-public.pem -rawin -in checksums.txt -sigfile checksums.txt.sig
if ($LASTEXITCODE -ne 0) { throw 'Invalid release signature' }
$asset = 'motd-windows-amd64.exe'
$entry = @(Get-Content checksums.txt | Where-Object { $_ -match '^[0-9a-f]{64}  motd-windows-amd64\.exe$' })
if ($entry.Count -ne 1 -or (Get-FileHash $asset -Algorithm SHA256).Hash -ne $entry[0].Substring(0,64)) { throw 'Checksum mismatch' }
$dir = Join-Path $env:LOCALAPPDATA 'Programs\motd'
New-Item -ItemType Directory -Force $dir | Out-Null
Copy-Item $asset (Join-Path $dir 'motd.exe') -Force
& "$dir\motd.exe" -v
```

Add that directory to your user `Path` to run `motd` from new shells.

### Build from source

See [Build](../README.md#build) for the pinned toolchain and build command.
Run `bin/motd` directly; building does not replace any installed package.

## Configuration

`motd` supports JSON config only. The config file is optional; without it, `motd` displays system information and skips media integrations.

Config lookup order:
1. `~/.config/motd/config.json`
2. `/opt/motd/config.json`

Use `motd -config /path/to/config.json` to select an existing JSON file, or
`motd -no-config` to skip config loading. `motd check-config` accepts a missing
default configuration; an explicitly requested missing file is an error.

Legacy YAML is not loaded; see [Migrating to MOTD 2.0](../MIGRATE_v2.md) for
manual field mappings. The interactive wizard can write a JSON config to a
default or explicit path:

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

Optional integrations:
- `vnstat` supplies monthly bandwidth figures on Linux/macOS when data is available.
- A separately running `motd-status-agent` provides rootless container status over a configured Unix socket. `motd` does not launch the agent.
- `who` provides a logged-in user count on Unix; `figlet` can render the hostname banner.

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

To show the report in interactive Bash or zsh sessions, add this line once to
`~/.bashrc` or `~/.zshrc`. Use the installed command's full path if your shell
profile runs before its directory is added to `PATH`.

```sh
case $- in *i*) motd ;; esac
```

## Troubleshooting

### Command Not Found

On Linux/macOS, use `type -a motd` to inspect shell resolution. Check the
directory you installed to and add it to `PATH` if necessary. On Windows,
use `Get-Command motd -All` or run the installed `motd.exe` by full path.

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

For `go-motd` packages, follow [APT removal](../INSTALL.md#remove). For a
standalone install, remove only the `motd` or `motd.exe` file from the directory
you chose. Remove any profile line you added. Neither removal requires deleting
configuration, which can contain credentials.

## Roll back

For APT, use `apt-cache policy go-motd` to list available versions. Install the
chosen package version with `sudo apt install go-motd=VERSION-1` (substitute an
actual version), then confirm `/usr/bin/motd -v`. A later upgrade can update it again.

For a standalone binary, select a known-good tag from
[Releases](https://github.com/thewildhive/go-motd/releases). The installer and
`self-update` select only the latest release. On Windows repeat the signed
installation steps with that tag. On Linux/macOS, download the raw binary for
your platform, `checksums.txt` and `checksums.txt.sig` from that same tag into
an empty directory. Save the trusted release key as `release-public.pem` as
described under [Windows](#windows-amd64), then verify and install. This example
is for a per-user Linux arm64 installation; set `asset` to your exact filename:

```bash
(
  set -eu
  asset=motd-linux-arm64
  openssl pkeyutl -verify -pubin -inkey release-public.pem -rawin -in checksums.txt -sigfile checksums.txt.sig
  expected=$(awk -v name="$asset" '$2 == name {print $1}' checksums.txt)
  actual=$(openssl dgst -sha256 "$asset" | awk '{print $NF}')
  [[ "$actual" =~ ^[0-9a-f]{64}$ && "$actual" = "$expected" ]] || exit 1
  install -m 0755 "$asset" "$HOME/.local/bin/motd"
  "$HOME/.local/bin/motd" -v
)
```

## Help

- Issues: <https://github.com/thewildhive/go-motd/issues>
- Releases: <https://github.com/thewildhive/go-motd/releases>
