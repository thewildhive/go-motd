# Install go-motd with APT

The signed APT repository supports Debian and Ubuntu on **amd64 and arm64**.
The package is named `go-motd`; the command is `motd`, installed at `/usr/bin/motd`.

Already using a standalone binary? Follow [Migrate.md](Migrate.md) to install the
package and remove the old binary without a backup. APT does not remove files
from `/usr/local/bin`, which can otherwise hide the package-installed command.
For macOS, Windows, or standalone installation, see the
[platform installation guide](docs/INSTALL.md).

## Install

Run the commands in Bash as your normal user with sudo access. If you use zsh
with custom pre-command hooks, first start `bash --noprofile --norc` and paste
the commands there.

Install the tools needed to download and verify the public signing key:

```bash
sudo apt update
sudo apt install ca-certificates curl gnupg
```

Configure the repository and install the package. The fingerprint check must
succeed before the key is installed. Do not bypass it or use `trusted=yes`.
The public key is trusted only for this source through `Signed-By`.

```bash
(
  set -eu
  arch=$(dpkg --print-architecture)
  case "$arch" in
    amd64|arm64) ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  base=https://thewildhive.github.io/go-motd
  curl -fLSs "$base/go-motd-archive-keyring.gpg" -o "$work/key.gpg"
  actual=$(gpg --batch --show-keys --with-colons "$work/key.gpg" |
    awk -F: '$1 == "fpr" {print $10; exit}')
  test "$actual" = '13AADCCA9CA0F6C3D3000317AE8EEB3EDD382604'
  sudo install -d -m 0755 /etc/apt/keyrings
  sudo install -m 0644 "$work/key.gpg" /etc/apt/keyrings/go-motd.gpg
  printf '%s\n' \
    'Types: deb' "URIs: $base/" 'Suites: stable' 'Components: main' \
    "Architectures: $arch" 'Signed-By: /etc/apt/keyrings/go-motd.gpg' \
    > "$work/go-motd.sources"
  sudo install -m 0644 "$work/go-motd.sources" /etc/apt/sources.list.d/go-motd.sources
  sudo apt update
  sudo apt install go-motd
)
```

Repeating the setup replaces the same key/source files; it does not change your
MOTD configuration. Existing package installations do not need reinstalling or
a separate migration just to enable APT updates.

## Verify and configure

```bash
hash -r
type -a motd
/usr/bin/motd -v
motd -v
apt-cache policy go-motd
motd -no-config
```

Both version commands should agree. APT policy should list
`https://thewildhive.github.io/go-motd` for your architecture. If `motd` runs an
older version, follow [the standalone cleanup](Migrate.md#3-remove-the-old-binary-without-a-backup).
After returning from Bash to zsh, run `rehash` to clear cached command locations.

Configuration is optional. Existing `~/.config/motd/config.json` and
`/opt/motd/config.json` remain intact. Run `motd configure` as your normal user
if you want media integrations; keep credentials out of public dotfiles.
See [configuration and shell integration](docs/INSTALL.md#configuration).
Package installation does not add a login hook or start a service.

## Update

```bash
sudo apt update
sudo apt install --only-upgrade go-motd
```

Normal `sudo apt upgrade` also includes go-motd. Do not use `motd self-update`
or the standalone installer for a package-managed installation. Unattended
upgrades require separately allowing origin `go-motd`; this setup does not
enable them.

Release workflows build both architecture packages, verify their signed
checksums, and deploy signed APT metadata to GitHub Pages. Metadata refreshes
weekly and expires after 30 days. For signature, expiry, or transient index-hash
errors, see [APT troubleshooting](docs/INSTALL.md#apt-repository); never disable
authentication to work around an error.

## Remove

```bash
sudo apt remove go-motd
```

This leaves user configuration intact. If you previously migrated a standalone
installation, remove `/usr/local/bin/motd` only after confirming it is the
compatibility symlink to `/usr/bin/motd`. To stop using this repository, remove
`/etc/apt/sources.list.d/go-motd.sources` and `/etc/apt/keyrings/go-motd.gpg`, then
run `sudo apt update`. Do not delete configuration directories as part of removal.
