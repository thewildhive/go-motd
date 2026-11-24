# Reboot marker

## Summary

Show when the local Linux environment has requested a reboot through the conventional Debian/Ubuntu reboot marker. This is a read-only signal available to normal users, not a general reboot assessment.

## Behavior

1. A normal `motd` invocation displays `Reboot` as a labeled row in System Information when a regular reboot marker exists. The value is `Required`, aligned with the other system values and highlighted when color is enabled. The check requires no configuration or elevated privileges.
2. For the same positive signal, `motd --json` adds `system.reboot_required: true` (a JSON boolean). Both formats omit the signal when the marker is absent or cannot be checked, or on unsupported platforms. Omission does not mean the system is up to date or safe to leave running without a reboot.
3. Missing markers are normal. Unexpected lookup errors or malformed markers are reported only in debug mode, without changing the exit status, suppressing other information, or polluting JSON stdout.
4. Each report observes the marker anew. The check does not read package lists or file contents, modify the marker, request a reboot, or infer host status from inside a container. It does not run for help, version, or configuration-only commands.
