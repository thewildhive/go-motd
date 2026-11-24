# Migrate to Debian packages

Use this guide on Debian/Ubuntu amd64 or arm64 to replace a standalone `motd`
installation with the `go-motd` package and receive updates through APT.
The cleanup deletes the old binary without making a backup. Configuration at
`~/.config/motd/config.json` and `/opt/motd/config.json` is not changed or deleted.

## 1. Use Bash and check the current installation

If you use zsh, start a clean Bash session before pasting the commands:

```sh
bash --noprofile --norc
```

Run the remaining commands inside that Bash session. This avoids custom zsh
pre-command hooks interfering with pasted multiline commands.

```bash
type -a motd
ls -l /usr/local/bin/motd /usr/bin/motd
```

A missing `/usr/bin/motd` is normal before package installation. If `motd` is an
alias, function, or executable in another directory, inspect that definition or
path separately; the cleanup below only handles `/usr/local/bin/motd`.

## 2. Configure APT and install the package

Follow the [APT installation steps](INSTALL.md#install), including verification
of the signing key. If a release `.deb` is already installed, configure the same
source for updates; there is no need to uninstall the package.

Stop if installation fails. Confirm the package binary works before cleanup:

```bash
dpkg-query -W -f='${Status} ${Version}\n' go-motd
/usr/bin/motd -v
apt-cache policy go-motd
```

Expect `install ok installed`, version 3.x or newer, and the GitHub Pages source.
An installed package alone does not mean your shell is using it:
`/usr/local/bin` commonly takes precedence over `/usr/bin`.

## 3. Remove the old binary without a backup

This replaces the default standalone path with a symlink to `/usr/bin/motd`.
Existing login hooks using `/usr/local/bin/motd` continue to work, and APT owns
the actual executable. Run the following as your normal user. It checks the old
binary before using sudo to change files, and refuses custom symlinks and
package-owned files. It can be rerun after a successful migration.

```bash
bash -eu -o pipefail <<'SH'
test "$(dpkg-query -W -f='${Status}' go-motd)" = 'install ok installed'
/usr/bin/motd -v
old=/usr/local/bin/motd
if dpkg-query -S "$old" >/dev/null 2>&1; then
  echo "Refusing to replace a package-owned file: $old" >&2
  exit 1
fi
if [ -L "$old" ]; then
  test "$(readlink "$old")" = /usr/bin/motd || {
    echo "Refusing to replace a custom symlink: $old" >&2
    exit 1
  }
elif [ -e "$old" ]; then
  test -f "$old"
  test -x "$old"
  "$old" -v | grep '^MOTD Script v'
  sudo rm -- "$old"
fi
sudo install -d -m 0755 /usr/local/bin
if [ ! -L "$old" ]; then
  sudo ln -s /usr/bin/motd "$old"
fi
SH
hash -r
motd -v
ls -l /usr/local/bin/motd
```

Expect `/usr/local/bin/motd -> /usr/bin/motd` and the same version from `motd -v`
and `/usr/bin/motd -v`. If you started Bash from zsh, run `exit`, then `rehash`
in zsh and check `motd -v` again.

If you still see an old version or a GitHub update notice, inspect `type -a motd`
and your shell/login hooks for aliases or other absolute binary paths. Do not
delete `/bin/motd`: on merged-/usr systems it is the same package-owned file as
`/usr/bin/motd`.

## 4. Optionally delete backups from earlier migrations

This guide creates no binary backup. An earlier `install-deb.sh --migrate` or
standalone self-update may have left backups. Inspect the exact files first:

```bash
sudo find /usr/local/bin -maxdepth 1 -type f \
  \( -name 'motd.pre-deb.*' -o -name 'motd.backup' \) -print
```

After confirming a listed file is an unwanted old MOTD binary, remove that exact
path with `sudo rm -- /usr/local/bin/EXACT_FILENAME`. Do not use broad deletion
patterns or remove configuration directories.

## Future updates

```bash
sudo apt update
sudo apt install --only-upgrade go-motd
```

Normal `apt upgrade` also includes go-motd. Stop using `motd self-update` and the
standalone installer for this installation. See the
[APT guide](INSTALL.md#remove) for repository removal, or [APT troubleshooting](docs/INSTALL.md#apt-repository)
for signature and expiry errors. The older `install-deb.sh VERSION --migrate`
helper backs up a recognized standalone binary before replacing it; unlike the
APT setup, the helper only installs the specified release and does not add a source.
