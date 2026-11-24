# Reboot marker implementation

## Context

[PRODUCT.md](PRODUCT.md) defines the signal. On fetched main, [main.go](https://github.com/thewildhive/go-motd/blob/eb7cc448fab5485656f573e2640011c6fde72ef9/main.go#L90-L108) splits terminal and JSON rendering, and [report.go](https://github.com/thewildhive/go-motd/blob/eb7cc448fab5485656f573e2640011c6fde72ef9/report.go#L78-L88) builds the JSON system report.

## Proposed changes

- Put the probe in `system`. Guard it with `runtime.GOOS == "linux"` so non-Linux builds do not touch either path. Use `os.Stat` for `/run/reboot-required`; only on `fs.ErrNotExist` try `/var/run/reboot-required` for legacy layouts. The paths are usually aliases. Accept a regular file (including a symlink resolving to one); size and contents are irrelevant. A non-regular target or unexpected error makes the result unavailable; log through the existing debug path to stderr. No package manager, dependency, subprocess, or privilege escalation.
- Use the same probe once per report in either renderer. Display the positive result with the existing warning color in System Information; in JSON use an optional boolean (`omitempty`). No separate command, flag, config, or cache.
- Metadata lookup requires search permission on parent directories, not read permission on the marker itself. On this workstation, UID 1000 got `No such file or directory` for `/run/reboot-required` and successfully statted a temporary regular file with mode `000`. No live marker was present; no system files were modified.

## Validation

- For behaviors 1-3, run the actual CLI as an unprivileged user in a disposable Linux environment. Set up its runtime directory from outside the CLI: check absent, mode `000` regular marker, legacy path only, non-regular marker, and permission denial. Inspect terminal output and parse JSON; compare normal and debug stderr. Remove the marker and rerun for behavior 4. Capture the terminal output and JSON as proof.
- Cross-compile for the repository's supported Windows and macOS targets; verify omission with the native CLI on a non-Linux host. No automated test suite is required.

## Parallelization

One implementation owner: the probe and two call sites are small and share one contract.
