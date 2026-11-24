# Reboot marker implementation

## Implementation

[PRODUCT.md](PRODUCT.md) defines the signal. [The probe](../../system/reboot.go) runs only on Linux. It uses `os.Stat` on `/run/reboot-required` and tries `/var/run/reboot-required` only if the first lookup reports `fs.ErrNotExist`. A regular file, including a symlink resolving to one, is sufficient. File size, contents, and read permissions are irrelevant; the process needs search permission on the parent directories. A non-regular target or an unexpected error suppresses the signal and produces a diagnostic only in debug mode.

Both report modes check once per invocation, without a subprocess or cache. [Terminal rendering](../../main.go) uses the shared dotted-label format for `Reboot` and the warning color for `Required`. [JSON rendering](../../report.go) sets `system.reboot_required` only when true; it does not include terminal formatting.

## Validation

- For behaviors 1-3, run the actual CLI as an unprivileged user in a disposable Linux environment. Set up its runtime directory from outside the CLI: check absent, mode `000` regular marker, legacy path only, non-regular marker, and permission denial. Inspect terminal output and parse JSON; compare normal and debug stderr. Remove the marker and rerun for behavior 4. Capture the terminal output and JSON as proof.
- Cross-compile for the repository's supported Windows and macOS targets; verify omission with the native CLI on a non-Linux host. No automated test suite is required.
