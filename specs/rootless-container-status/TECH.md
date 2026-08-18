# Rootless Container Status Technical Design

## Context

The product behavior is defined in [PRODUCT.md](PRODUCT.md). At commit `62d971ccd2ae8810c1f4d9ba49e02cef9a86e697`, `go-motd` invokes Docker directly in [`system/system.go:20-41`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/system/system.go#L20-L41), discovers Compose files and shells out per project in [`system/compose.go:25-115`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/system/compose.go#L25-L115), and calls both renderers from [`main.go:97-110`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/main.go#L97-L110). JSON reporting independently calls the Compose collector in [`report.go:68-85`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/report.go#L68-L85).

Configuration is strictly decoded in [`config/config.go:25-40,73-90`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/config/config.go#L25-L40). The wizard edits system settings in [`configure.go:64-73`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/configure.go#L64-L73), and diagnostics validate `compose_dir` in [`check_config.go:32-85`](https://github.com/thewildhive/go-motd/blob/62d971ccd2ae8810c1f4d9ba49e02cef9a86e697/check_config.go#L32-L85). The module uses only the standard library and must preserve that constraint.

## Proposed Changes

### Shared Protocol

Protocol v1 is HTTP/1.1 over a Unix stream socket. The only successful operation is `GET /v1/status`; no request body is sent. The client sends `Host: localhost` and `Accept: application/json`.

A `200 OK` body is:

```json
{
  "protocol_version": 1,
  "observed_at": "2026-08-18T12:34:56.123456789Z",
  "workloads": [
    {
      "name": "jellyfin",
      "state": "running",
      "health": "healthy"
    }
  ]
}
```

Required fields reject missing, null, or invalid values. `observed_at` uses RFC 3339 with optional fractional seconds. Workload names must be non-empty and unique. `state` accepts only `running`, `stopped`, `failed`, or `unknown`; `health` accepts only `healthy`, `unhealthy`, `starting`, `none`, or `unknown`. Unknown JSON fields are ignored for additive v1 compatibility. The agent supplies no aggregate counts; the consumer sorts workloads by name and derives `online` as `state == "running" && (health == "healthy" || health == "none")`.

Non-200 responses use JSON when possible:

```json
{
  "protocol_version": 1,
  "error": {
    "code": "collection_timeout",
    "message": "workload status collection timed out"
  }
}
```

Stable v1 error codes are `bad_request`, `not_found`, `method_not_allowed`, `runtime_unavailable`, `permission_denied`, `collection_timeout`, `response_too_large`, and `internal_error`. Clients treat every non-200 response as unavailable; the codes exist for bounded debug diagnostics.

### Configuration and Migration

- Replace `System.ComposeDir` with `System.ContainerStatus`, containing `SocketPath string` (`socket_path`) and `MaxAge string` (`max_age`). Keep duration text in persisted config and parse it with `time.ParseDuration`; an empty max age means `30s`.
- Use `/var/run/motd-status/agent.sock` as the wizard's offered default and the effective path when an enabled `container_status` object omits `socket_path`. An absent `container_status` object remains disabled.
- Add a decode-time migration check for `system.compose_dir` before strict decoding. Return a dedicated error explaining that Compose polling was removed and naming `system.container_status.socket_path`. Do not retain `ComposeDir` in the runtime config type.
- Update sample configuration, README, installation and migration documentation. Remove Docker/Compose optional-tool claims and describe the agent as an external prerequisite.

### Client and Collection

- Replace `system/compose.go` with `system/container_status.go`. Define exported `ContainerStatus` and `WorkloadStatus` values used by terminal and JSON renderers. Keep protocol DTOs private.
- Implement a local `http.Client` with a custom `http.Transport.DialContext` that always dials the configured Unix socket. Set the client timeout to one second and disable redirects. Do not use a package global client or transport.
- Limit the response body to 1 MiB plus one sentinel byte before decoding. Reject oversized data, non-200 status, non-JSON content, malformed JSON, trailing JSON values, duplicate names, invalid enums, unsupported protocol versions, stale timestamps, and timestamps over five seconds in the future.
- Derive counts only after validating the complete response. Sort workloads by name. Return `(ContainerStatus, true)` for a valid empty response and `false` for unavailable data so JSON can distinguish these states.
- Delete `system/compose.go` and `system/compose_test.go`, remove `ShowDocker`, and remove `ComposeDir` from `system.ConfigAccessor`.

### Rendering and Diagnostics

- Add `ShowContainers(cfg, debug)` and call it once where the two Docker renderers currently run. It emits no line for an empty list, green `All workloads online` when all are online, and yellow `X of Y online` otherwise.
- Replace `outputReport.Compose` with an optional `Containers` report carrying protocol version, observation timestamp, derived counts/status, and complete sorted workload details including derived `online`.
- Extend the wizard's System Setup section with an enable/edit prompt, socket path default, and max-age default. Disabling clears both fields.
- Extend `check-config`: parse duration and require it to be positive; require an absolute socket path; inspect existing entries with `os.Lstat`; require a Unix socket; and attempt a bounded protocol request to distinguish permission/connectivity errors. A nonexistent socket is a warning. Protocol or permission failures are errors because a present endpoint cannot be consumed.
- Gate Unix-socket communication by platform. On Windows, configured container status produces an actionable validation error and normal output skips it. Darwin can use Unix sockets but the external agent remains Linux-only; cross-platform compilation must retain all symbols.

## Testing and Validation

- Config tests cover absent/default values, valid status config, malformed durations, strict unknown fields, and the targeted `compose_dir` migration error (Behavior 1-6, 24).
- Client tests use a temporary Unix listener and HTTP server to cover success, empty lists, every state/health combination, deterministic sorting, additive fields, duplicate names, malformed/trailing JSON, non-200 error bodies, unsupported versions, 1 MiB limits, stale/future timestamps, timeout, disconnect, and unavailable socket (Behavior 7-11, 14-21, 27).
- Rendering tests capture output for all-online, degraded, empty, and unavailable snapshots and assert names never appear in terminal output (Behavior 12-14, 19, 23, 26).
- JSON tests assert the complete schema and every workload's derived online value, including an empty configured source and omission on failure (Behavior 14-16, 19).
- Wizard and diagnostics tests cover enable/edit/remove flows, defaults, path type, missing socket warning, permission/connect failures, invalid max age, and unsupported platforms (Behavior 2-6, 22).
- Documentation and repository searches verify no active `compose_dir`, `ShowDocker`, Docker Compose polling, or obsolete config example remains (Behavior 23-26).
- Run `gofmt`, `go vet ./...`, `go test ./... -count=1`, race tests, and Linux/Windows/Darwin amd64 plus Darwin arm64 builds through `make check` or the equivalent required commands.
- Perform one Linux integration check against a protocol fixture and, after the agent exists, one end-to-end check against its real Unix socket.
- The cross-repository check is `make test-status-agent-integration`. It builds `../motd-status-agent/cmd/motd-status-agent-fixture` and `go-motd`, serves the agent repository's canonical `testdata/v1/status.json` through the real Unix-socket HTTP server, asserts 2 of 5 online plus JSON workload details, checks aggregate-only terminal output, and verifies omission after socket shutdown.

## Risks and Mitigations

- Immediate `compose_dir` removal breaks existing configuration. The dedicated migration error and documentation make the required action explicit.
- A shared `/var/run` socket requires administrator-managed ownership. The consumer never creates or changes it and treats absence as best-effort.
- Login latency can regress if socket operations hang. The single one-second client deadline, body limit, and no-retry rule bound the impact.
- Protocol drift across repositories is controlled by copying the v1 schema and contract tests into both technical specs; incompatible changes require protocol version 2.

## Parallelization

Implementation can use two local agents after the protocol DTO and config shape are landed sequentially on `feat/rootless-container-status`.

- `consumer-core`: owns `config/`, `system/container_status.go`, deletion of Compose probing, and protocol/config tests. Use `/home/calmcacil/worktrees/go-motd-consumer-core` on branch `feat/rootless-container-status-core`.
- `consumer-surfaces`: owns `configure.go`, `check_config.go`, `main.go`, `report.go`, documentation, and their tests. Use `/home/calmcacil/worktrees/go-motd-consumer-surfaces` on branch `feat/rootless-container-status-surfaces` after the shared config/status types are available.

Merge core into the feature branch first, rebase surfaces onto it, resolve only integration points, then run the authoritative cross-platform gate on the combined branch. Land as one PR because partial consumer surfaces are not useful independently.
