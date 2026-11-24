# Security Policy

## Supported versions

Security fixes target the latest stable release. Fixes and release metadata go through pull requests against protected `main`.

## Reporting a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/thewildhive/go-motd/security/advisories/new). If it is unavailable, contact the maintainers privately through the contact information on the repository profile. Do not open a public issue for a suspected vulnerability.

Include a clear description, affected version, reproduction steps or proof of concept, and any suggested mitigation. We will acknowledge reports within seven days and coordinate disclosure after a fix is available.

## Release integrity

- **Raw binaries:** `self-update` verifies the Ed25519 signature on `checksums.txt` with its embedded public key, then checks the downloaded binary's SHA-256 digest.
- **Archives and `.deb` release assets:** verify `archive-checksums.txt.sig` against `archive-checksums.txt` before checking the downloaded file's SHA-256 digest. Do not install if either check fails.
- **APT:** the publisher verifies the Debian release assets before adding them to the repository. APT authenticates repository metadata with a separate OpenPGP key. Follow the [APT setup instructions](docs/APT-REPOSITORY.md) to pin that trust to this repository.

Release coordination uses a short-lived GitHub App token. The release and APT publishing jobs use separate signing credentials (`SIGNING_PRIVATE_KEY` and `APT_SIGNING_KEY`); pull-request jobs receive none of them. If a credential may have been exposed, suspend the affected publisher, revoke or rotate the credential, and update the corresponding secret. For a signing-key change, coordinate distribution of the new trusted public key before resuming publication. Never overwrite a published asset to repair it.
