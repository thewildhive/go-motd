# Daily Drive Health in the MOTD

## Summary

Show Linux operators one System row answering: are all monitored drives known healthy, and if not, which drives need attention? `motd-status-agent` owns collection and classification. `go-motd` only reads the published snapshot; login never runs `smartctl`, opens a drive, requests a refresh, or waits for the collector.

## Behavior

1. Drive health is off by default. When disabled, do not read the snapshot, show a row, or add a JSON field. On Linux, `system.drive_health: true` enables the report from `/var/lib/motd-drive-health/status.json`. No other system row or container status changes.
2. Use the existing System row style: `Drives` as the dot-padded label, the usual colon and space, then one value. If every drive in a valid fresh snapshot is `healthy`, the entire value is `healthy`. Do not list drives or counts.
3. If any fresh row is `warning`, `failed`, or `unknown`, start the value with `degraded: ` and list only those drives, in label order, separated by comma and space. Include each label and its reported state, for example `degraded: Archive (unknown), Cache (failed), Mirror (warning)`. Unknown means health could not be established, not that the disk failed. Keep one logical row; allow terminal wrapping rather than dropping labels. The words carry the same meaning without color.
4. A valid snapshot more than 36 hours old, measured from its UTC `observed_at` rather than file modification time, yields `stale`. Missing, unreadable, unsafe, malformed or unsupported snapshots yield `unavailable`. Neither value lists drives or implies a current hardware fault. Leave the rest of the MOTD usable. Debug mode explains rejection without exposing raw device data; the next fresh valid snapshot restores normal reporting.
5. Accept only the collector's v1 snapshot: at least one drive, distinct display labels of 1–32 printable ASCII characters with no surrounding spaces, valid state/reason pairs and a valid observation timestamp no more than five minutes ahead. Never print unvalidated labels or control characters. Read only a root-owned regular file in the trusted state directory, not a symlink or a group/world-writable file. Reject more than 64 KiB without unbounded reads; a FIFO without a writer must not stall login.
6. When enabled, JSON includes top-level `drive_health` with `status` (`fresh`, `stale`, or `unavailable`) and `observed_at` when known. Only `fresh` includes `drives`: all validated `{label, state, reason}` rows, including healthy ones. `degraded` is a terminal summary of fresh rows, not a fourth freshness status. Neither output exposes serials, raw device paths, or telemetry.

## Scope

This specification replaces the producer proposal's count-based terminal example for `go-motd` only. It does not change the snapshot format, collector, timer, disk permissions, or polling ownership.
