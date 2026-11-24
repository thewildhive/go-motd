# Rootless Container Status Technical Design

## Consumer protocol

[PRODUCT.md](PRODUCT.md) defines the user-visible contract. The [client](../../system/container_status.go) sends `GET /v1/status` over the configured Unix stream socket using HTTP/1.1, `Host: motd-status-agent`, and `Accept: application/json`. There is no request body or authentication token; the transport dials only the local socket, not the HTTP hostname.

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

The client requires `200 OK` and media type `application/json`. It rejects other status codes without interpreting their body as an agent error message. There are no redirects, retries, or cached snapshots. The client applies a one-second timeout, reads at most 1 MiB plus one overflow byte, and rejects oversized or malformed responses. An observation older than `max_age` or more than five seconds in the future is unavailable.

## Configuration and migration

[Configuration loading](../../config/config.go) rejects the removed `system.compose_dir` before strict decoding. The diagnostic names `system.container_status.socket_path` and `motd-status-agent`. A missing `container_status` object disables requests. With an object present, the default socket is `/var/run/motd-status/agent.sock`; the default maximum age is `30s`. Overrides require an absolute path and a positive Go duration.

[The wizard](../../configure.go) adds, edits, or removes the object. [Configuration diagnostics](../../check_config.go) check the settings and inspect the socket with `os.Lstat`. A missing entry is a warning; a non-socket or symlink is an error. For an existing socket, validation requests the same current snapshot used for reporting. Failure to obtain one is an error. There is no separate Windows validation path; a usable Unix socket is required on the host that runs the client.

## Rendering

[Terminal output](../../main.go) uses a dotted `Containers` label in Services & Resources. It shows green `All workloads online` or yellow `X of Y online`, without listing names. It omits the row when no workloads were reported or no valid snapshot was available.

[JSON output](../../report.go) emits an optional `containers` object with `protocol_version`, UTC `observed_at`, `online`, `total`, `status`, and `workloads` sorted by name. Each workload includes its `name`, `state`, `health`, and derived `online` value. An empty response has status `0 workloads` and an empty array. An unavailable source is omitted, and the rest of the report continues.

## Validation

- Build with `make build`. On a host with the real status agent, use `./bin/motd configure` to enable the socket in a local config, then run `./bin/motd check-config` as the account that reads the MOTD. Record the diagnostics, including any path or permission errors.
- Compare `./bin/motd -no-color` and `./bin/motd -json` with the workloads actually served by the agent. The terminal shows only the aggregate for a nonempty snapshot; JSON includes the sorted names, health, and derived online values. A valid empty snapshot is present in JSON but omitted from terminal output.
- Point a separate config at a nonexistent absolute socket path and run both report modes with `-config /path/to/config.json`. Confirm container status is omitted, not reported as zero, and other system information still appears.
- Run `make check` for static analysis and compilation on all five release targets. Attach the relevant CLI commands and output as runtime evidence.

## Risks and Mitigations

- Existing `compose_dir` configurations require migration. The dedicated error names the new setting.
- A shared `/var/run` socket requires administrator-managed ownership. The consumer never creates or changes it and treats absence as best-effort.
- Login latency can regress if socket operations hang. The single one-second client deadline, body limit, and no-retry rule bound the impact.
- Additive fields are compatible with protocol v1; an incompatible change requires a new protocol version and a compatible consumer.
