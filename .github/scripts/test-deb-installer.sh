#!/usr/bin/env bash
# Exercise the real installer and apt in a disposable container with signed fixtures.
set -euo pipefail
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
arch="$(dpkg --print-architecture)"
bash .github/scripts/package-deb.sh 3.0.0 "$arch" "$work/assets"
openssl genpkey -algorithm ED25519 -out "$work/key.pem"
openssl pkey -in "$work/key.pem" -pubout -out "$work/public.pem"
(
  cd "$work/assets"
  sha256sum ./*.deb | sed 's|  ./|  |' > archive-checksums.txt
)
openssl pkeyutl -sign -inkey "$work/key.pem" -rawin \
  -in "$work/assets/archive-checksums.txt" -out "$work/assets/archive-checksums.txt.sig"
# Substitute only the trust anchor in the test copy; never change the shipping key.
awk -v key="$work/public.pem" '
  /^-----BEGIN PUBLIC KEY-----$/ { while ((getline line < key) > 0) print line; skip=1; next }
  /^-----END PUBLIC KEY-----$/ { skip=0; next }
  !skip { print }
' install-deb.sh > "$work/installer.sh"

docker run --rm --network host -i -v "$work:/fixtures:ro" ubuntu:24.04 bash -se <<'EOF'
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends curl openssl ca-certificates
printf 'APT::Get::Assume-Yes "true";\n' > /etc/apt/apt.conf.d/99test
mkdir -p /tmp/assets /tmp/mock /root/.config/motd /opt/motd
cp /fixtures/assets/* /tmp/assets/
cat > /tmp/mock/curl <<'MOCK'
#!/bin/bash
set -eu
while (($#)); do
  case "$1" in
    https://*) name="${1##*/}" ;;
    -o) shift; output="$1" ;;
  esac
  shift
done
cp "/tmp/assets/$name" "$output"
MOCK
chmod +x /tmp/mock/curl
export PATH="/tmp/mock:$PATH"
printf '#!/bin/sh\necho "MOTD Script v2.2.1 (test fixture)"\n' > /usr/local/bin/motd
chmod +x /usr/local/bin/motd
original="$(sha256sum /usr/local/bin/motd | cut -d ' ' -f1)"
printf 'user config\n' > /root/.config/motd/config.json
printf 'fallback config\n' > /opt/motd/config.json
config="$(sha256sum /root/.config/motd/config.json /opt/motd/config.json)"

# Neither a bad signature nor a modified package may install or touch the old binary.
for tamper in archive-checksums.txt.sig "go-motd_3.0.0-1_$(dpkg --print-architecture).deb"; do
  printf 'tampered' > "/tmp/assets/$tamper"
  if bash /fixtures/installer.sh 3.0.0 --migrate; then exit 1; fi
  test ! -e /usr/bin/motd
  test "$original" = "$(sha256sum /usr/local/bin/motd | cut -d ' ' -f1)"
  cp "/fixtures/assets/$tamper" "/tmp/assets/$tamper"
done
# A failed package install must also leave the standalone executable in place.
printf '#!/bin/sh\nexit 42\n' > /tmp/mock/apt
chmod +x /tmp/mock/apt
if bash /fixtures/installer.sh 3.0.0 --migrate; then exit 1; fi
test ! -e /usr/bin/motd
test "$original" = "$(sha256sum /usr/local/bin/motd | cut -d ' ' -f1)"
rm /tmp/mock/apt
bash /fixtures/installer.sh 3.0.0 --migrate
test "$(readlink /usr/local/bin/motd)" = /usr/bin/motd
test "$original" = "$(sha256sum /usr/local/bin/motd.pre-deb.* | cut -d ' ' -f1)"
dpkg-query -S /usr/bin/motd | grep -Fx 'go-motd: /usr/bin/motd'
motd -v | grep -F 'v3.0.0 '
bash /fixtures/installer.sh 3.0.0 --migrate
test "$(find /usr/local/bin -name 'motd.pre-deb.*' | wc -l)" -eq 1
test "$config" = "$(sha256sum /root/.config/motd/config.json /opt/motd/config.json)"
ln -sfn /bin/true /usr/local/bin/motd
if bash /fixtures/installer.sh 3.0.0 --migrate; then exit 1; fi
test "$(readlink /usr/local/bin/motd)" = /bin/true
echo 'Signed installer migration, repeat installation, and tamper rejection passed.'
EOF
