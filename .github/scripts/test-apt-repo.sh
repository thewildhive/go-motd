#!/usr/bin/env bash
set -euo pipefail
work="$(mktemp -d)"
export GNUPGHOME="$work/gnupg"
trap 'gpgconf --kill gpg-agent; rm -rf "$work"' EXIT
mkdir -m 0700 "$GNUPGHOME"
gpg --batch --pinentry-mode loopback --passphrase '' --quick-generate-key 'APT test only' ed25519 sign 1d
APT_SIGNING_FINGERPRINT="$(gpg --with-colons --list-keys | awk -F: '$1 == "fpr" {print $10; exit}')"
export APT_SIGNING_FINGERPRINT
for version in 3.0.0 3.0.1; do
  for arch in amd64 arm64; do
    bash .github/scripts/package-deb.sh "$version" "$arch" "$work/packages"
  done
done
bash .github/scripts/build-apt-repo.sh "$work/packages" "$work/site"
for arch in amd64 arm64; do
  index="$work/site/dists/stable/main/binary-$arch/Packages"
  [[ "$(grep -c '^Package: go-motd$' "$index")" == 2 ]]
  [[ "$(grep '^Architecture:' "$index" | sort -u)" == "Architecture: $arch" ]]
done
# Correctly signed but expired metadata must still fail APT authentication.
sed 's/^Valid-Until:.*/Valid-Until: Sat, 01 Jan 2000 00:00:00 +0000/' \
  "$work/site/dists/stable/Release" > "$work/expired-release"
gpg --batch --local-user "$APT_SIGNING_FINGERPRINT" --clearsign \
  -o "$work/expired-inrelease" "$work/expired-release"
docker run --rm --network none -i -v "$work:/fixtures:ro" ubuntu:24.04 bash -se <<'EOF'
export DEBIAN_FRONTEND=noninteractive
cp -r /fixtures/site /repo
chmod -R a+rX /repo
rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*
install -m 0644 /repo/go-motd-archive-keyring.gpg /usr/share/keyrings/go-motd.gpg
cat > /etc/apt/sources.list.d/go-motd.sources <<SOURCE
Types: deb
URIs: file:/repo
Suites: stable
Components: main
Architectures: $(dpkg --print-architecture)
Signed-By: /usr/share/keyrings/go-motd.gpg
SOURCE
apt-get update -o APT::Update::Error-Mode=any
apt-get install -y go-motd=3.0.0-1
mkdir -p /root/.config/motd /opt/motd
printf 'user config sentinel\n' > /root/.config/motd/config.json
printf 'fallback config sentinel\n' > /opt/motd/config.json
before="$(sha256sum /root/.config/motd/config.json /opt/motd/config.json)"

asset="go-motd_3.0.1-1_$(dpkg --print-architecture).deb"
printf 'tampered package' > "/repo/pool/main/g/go-motd/$asset"
if apt-get install -y --only-upgrade go-motd > /tmp/apt-error 2>&1; then exit 1; fi
grep -E 'Hash Sum mismatch|unexpected size' /tmp/apt-error
test "$(dpkg-query -W -f='${Version}' go-motd)" = 3.0.0-1
cp "/fixtures/site/pool/main/g/go-motd/$asset" "/repo/pool/main/g/go-motd/$asset"
apt-get install -y --only-upgrade go-motd
test "$(dpkg-query -W -f='${Version}' go-motd)" = 3.0.1-1
motd -v | grep -F 'v3.0.1 '
test "$before" = "$(sha256sum /root/.config/motd/config.json /opt/motd/config.json)"

sed -i 's/Origin: go-motd/Origin: tampered/' /repo/dists/stable/InRelease
rm -rf /var/lib/apt/lists/*
if apt-get update -o APT::Update::Error-Mode=any > /tmp/apt-error 2>&1; then exit 1; fi
grep -E 'BADSIG|invalid signature|not signed' /tmp/apt-error
cp /fixtures/expired-inrelease /repo/dists/stable/InRelease
rm -rf /var/lib/apt/lists/*
if apt-get update -o APT::Update::Error-Mode=any > /tmp/apt-error 2>&1; then exit 1; fi
grep 'expired' /tmp/apt-error
echo 'APT authenticated install/upgrade, architecture isolation, config preservation, tamper and expiry rejection passed.'
EOF
