# Security Policy

## Supported versions

Security fixes are applied to the latest release on `main`.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting feature for this repository. If it is unavailable, contact the maintainers privately through the contact information on the repository profile. Do not open a public issue for a suspected vulnerability.

Include a clear description, affected version, reproduction steps or proof of concept, and any suggested mitigation. We will acknowledge reports within seven days and coordinate disclosure after a fix is available.

## Release integrity

Release binaries are accompanied by SHA-256 checksums. `checksums.txt` is signed with the Ed25519 key embedded in the updater; do not trust a binary whose checksum or signature does not verify.

Release coordination uses a repository-scoped GitHub App installation token. Artifact signing remains a separate trust boundary using `SIGNING_PRIVATE_KEY`; neither credential is exposed to pull-request jobs. If either credential is suspected of exposure, disable publication, revoke or rotate it at its source, update the corresponding repository secret, and replace the embedded public key in a reviewed patch when the signing key changes.
