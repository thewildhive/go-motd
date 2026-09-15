# Maintaining the APT repository

The `Publish APT Repository` workflow deploys a complete signed repository to
GitHub Pages after `Publish Release` succeeds, on manual dispatch, and weekly.
The expected URL is `https://thewildhive.github.io/go-motd/`.

## Activation

1. In Settings > Pages, choose GitHub Actions as the publishing source.
2. Create a dedicated OpenPGP signing key on a trusted computer. Do not reuse
   the raw Ed25519 release-checksum key. Use an unattended signing key without a
   passphrase, with an expiry date, and retain an encrypted backup and revocation
   certificate outside GitHub.
3. Add the ASCII-armored private key as Actions secret `APT_SIGNING_KEY` and its
   uppercase 40-character fingerprint as Actions variable `APT_SIGNING_FINGERPRINT`.
4. Merge the workflow through a reviewed, passing PR, then manually run
   `Publish APT Repository` on `main` for the first deployment.
5. Verify the deployed key fingerprint against the original trusted value and
   test `apt update` and installation before sharing the [client setup](INSTALL.md#apt-repository).

The workflow imports the secret into a temporary GnuPG home and deletes it after
signing. Only public keys, metadata, and verified packages enter the Pages artifact.
Restrict repository write access: changes to publishing code can access signing
secrets. The workflow always checks out protected `main`, not triggering branch code.

## Publication and recovery

All stable releases starting at v3.0.0 must contain both architecture packages and
their signed `archive-checksums.txt`. The collector verifies the existing release
signature and each package checksum before repository construction. Missing or
invalid assets fail publication, leaving the last deployment intact. Repair a
failed release publisher and rerun this workflow; do not replace published assets.

Every deployment rebuilds from all retained GitHub releases. Never delete old
releases or assets: they are the repository's durable package store and provide
explicit version installation and rollback. As the archive approaches GitHub
Pages' size limits, migrate hosting rather than silently removing packages.

The `stable/main` indexes include amd64 and arm64 separately and retain all
versions. Publication is serialized across releases. A complete Pages artifact
is deployed only after signatures verify. CDN propagation can briefly produce
index hash mismatches; clients must retry, never bypass authentication.

Signed metadata expires in 30 days. The Monday 04:23 UTC refresh renews it even
without a new application release. Investigate failed refreshes before expiry;
GitHub can disable scheduled workflows after prolonged repository inactivity,
so check that this workflow remains enabled. Manual dispatch restores freshness.

## Key maintenance

Track the key expiry outside the repository and renew before it expires. For an
expiry extension, update the GitHub secret with the renewed key and redeploy;
clients also need the updated public key certificate. Distribute it through the
documented fingerprint-checked setup process.

Changing the primary key changes the fingerprint and requires an explicit client
trust migration. Do not simply replace the key and assume existing clients will
trust it. In a compromise, stop publication, revoke the key, and distribute the
replacement fingerprint through a trusted channel before resuming.

## Local verification

Run `make check` and `bash .github/scripts/test-apt-repo.sh`. The APT test needs
Go, GnuPG, apt-utils, dpkg-dev, and Docker. It uses throwaway keys and a disposable
Ubuntu container with no network access, testing authenticated installation and
upgrade, config preservation, architecture separation, and rejection of modified
packages, invalid signatures, and expired metadata. CI runs it on native amd64
and arm64 runners. Test keys must never be used for production publication.
