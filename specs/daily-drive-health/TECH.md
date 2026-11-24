# Daily Drive Health: Consumer Technical Design

## Ownership and configuration

The collector owns device probing, classification, scheduling, and atomic snapshot publication. This repository only consumes `/var/lib/motd-drive-health/status.json`; the login path does not invoke `smartctl`, request collection, or access a drive. Add `system.drive_health: true` as an explicit opt-in. The feature is disabled by default and does not affect container status or other system rows. Non-Linux builds provide an unavailable stub and do not use Linux file APIs.

## Snapshot contract

Accept only schema version 1 with a nonempty `drives` list and UTC RFC 3339 `observed_at`. Labels are 1–32 printable ASCII characters with no leading or trailing spaces, unique per snapshot. They are validated before any display or JSON output. State/reason pairs are exhaustive:

- `healthy`: `passed`
- `failed`: `overall_failed` or `nvme_critical`
- `warning`: `ata_prefail_now` or `nvme_advisory`
- `unknown`: `missing`, `identity_mismatch`, `access_denied`, `sleeping`, `unsupported`, `timeout`, `invalid_response`, or `probe_error`

Reject timestamps more than five minutes in the future. A valid snapshot older than 36 hours, measured using `observed_at` and not file modification time, is stale. Retain the validated observation time for stale reports; include drive rows only for fresh reports.

## Secure descriptor-based read

Open the fixed state directory and snapshot by descriptor, refusing a symlink at either final path component. Verify the opened directory is root-owned and not group/world writable. Open the snapshot with `O_NOFOLLOW | O_NONBLOCK`; inspect it with `fstat` and require a root-owned regular file with no group/world write permission and size at most 65,536 bytes. Read from that same descriptor through a 65,537-byte limit, rejecting overflow. This prevents FIFO stalls, path replacement races between validation and use, and unbounded reads.

Malformed, unknown-version, invalid, missing, inaccessible, unsafe, or oversized snapshots become `unavailable`. Debug output gives a rejection reason without printing snapshot contents. The MOTD continues normally in every failure case.

## Output

The terminal displays one dot-padded `Drives` row. Fresh snapshots with all healthy rows show `healthy`; otherwise show `degraded: ` followed by only non-healthy rows, sorted by label, formatted as `Label (state)` and comma-space separated. Stale and unavailable snapshots show only their status. JSON adds top-level `drive_health` only while opted in, with `status`, `observed_at` when known, and all validated `{label, state, reason}` rows only when fresh. It contains no device paths, serials, or raw telemetry.
