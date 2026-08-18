# Rootless Container Status

## Summary

`go-motd` can display container workload status supplied by a local, separately installed `motd-status-agent`, allowing MOTD to report rootless Podman workloads owned by another user without invoking that user's container runtime directly. The integration uses a runtime-neutral status contract so other container backends can provide the same experience in the future.

## Problem

The existing Docker and Compose status checks run commands as the user invoking `motd`. They cannot reliably inspect rootless Podman Quadlets owned by another user, and granting MOTD direct access to that user's runtime would unnecessarily couple it to the runtime and cross-user execution details.

## Goals

- Show an aggregate, health-aware container workload summary in terminal output.
- Make complete per-workload status available to JSON consumers, including Discord or other reporting integrations.
- Keep the integration local and least-privileged through a group-restricted Unix socket.
- Replace Docker-specific probing and naming with a runtime-neutral status source.

## Non-goals

- Installing, configuring, upgrading, or removing `motd-status-agent`; those behaviors belong to the agent repository's product spec.
- Starting, stopping, restarting, or otherwise managing workloads from `go-motd`.
- Connecting to a status agent over TCP or across a network.
- Discovering or invoking Podman, Docker, Compose, Quadlet, or another container runtime directly.

## Behavior

1. Container status is opt-in. When no `system.container_status` block is configured, `motd` performs no container-runtime or Compose probing and displays no container status line. When the block is configured without a socket override, `motd` uses `/var/run/motd-status/agent.sock`.

2. Users can configure:
   - The filesystem path of the local Unix socket exposed by `motd-status-agent`.
   - The maximum acceptable age of the agent's observation.

   The default socket path offered by configuration is `/var/run/motd-status/agent.sock`.

3. The default maximum observation age is 30 seconds. Users may override it with a positive duration through configuration.

4. `motd configure` allows users to add, edit, or remove the status-agent socket path and maximum observation age. Removing the socket configuration disables container status without requiring an agent to be uninstalled.

5. `motd check-config` validates that:
   - The configured socket path is absolute.
   - The configured maximum age is a valid positive duration.
   - If a filesystem entry currently exists at the configured path, it is a Unix socket rather than a regular file or directory.
   - The invoking user can connect to an existing socket.

6. A missing socket is reported as a warning by `motd check-config`, not as an invalid configuration, because the user service may be temporarily stopped. Permission denial, a non-socket entry, an invalid path, and an invalid maximum age produce actionable diagnostics that identify the affected setting.

7. When the socket is configured, normal terminal and JSON output request one current status document from the agent during each invocation. Filesystem permissions are the only authentication mechanism; `go-motd` does not send or require a token.

8. The agent response provides, at minimum:
   - A protocol version.
   - The time at which workload status was observed.
   - A list of logical workloads.
   - A stable name for each workload.
   - A normalized runtime state for each workload: `running`, `stopped`, `failed`, or `unknown`.
   - A normalized health state for each workload: `healthy`, `unhealthy`, `starting`, `none`, or `unknown`.

9. The status contract counts logical workloads, not individual underlying containers. A pod, service, or other logical unit represented by the agent appears once even when it contains multiple containers.

10. A workload is online when its runtime state is `running` and its health is either `healthy` or `none`. A workload is not online when it is stopped, failed, unknown, unhealthy, still starting, or has unknown health.

11. The aggregate total equals the number of workloads in the response, and the aggregate online count is derived from the rules in (10). `go-motd` does not trust conflicting aggregate counts supplied by an agent in place of the workload list.

12. When every reported workload is online, terminal output shows one runtime-neutral line labeled `Containers` with `All workloads online` in the existing success style.

13. When one or more reported workloads are not online, terminal output shows one line labeled `Containers` with `X of Y online` in the existing degraded style. Terminal output never includes workload names, regardless of how many workloads are degraded.

14. A successful response containing zero workloads produces no `Containers` line in normal terminal output. JSON output represents the configured source with an empty workload list, zero counts, and an explicit empty-source status so machine consumers can distinguish it from an unavailable source.

15. JSON output exposes:
   - The observation timestamp.
   - The aggregate online and total counts.
   - A human-readable aggregate status consistent with terminal output.
   - Every workload's stable name, normalized runtime state, normalized health state, and derived online value.
   - The protocol version returned by the agent.

16. JSON workload order is deterministic. Workloads are ordered by stable name so consumers do not observe changes caused only by runtime enumeration order.

17. `go-motd` treats a response as stale when its observation timestamp is older than the configured maximum age at the time the response is received. An observation timestamp more than 5 seconds later than the receipt time is invalid rather than fresh.

18. An unavailable socket, permission denial, timeout, malformed response, unsupported protocol version, missing required field, unknown normalized status value, stale response, future timestamp, or interrupted request makes container status unavailable for that invocation.

19. Unavailable container status is best-effort in normal operation:
   - Terminal output omits the `Containers` line.
   - JSON output omits the container-status section rather than emitting potentially misleading data.
   - The rest of MOTD and JSON reporting continues normally.
   - Debug output explains the reason without exposing unrelated sensitive response content.

20. Agent communication has a 1-second total timeout so a stopped or wedged user service cannot noticeably delay login. A timeout does not trigger a retry during the same invocation. Responses larger than 1 MiB are rejected.

21. `go-motd` does not cache agent responses. If the agent is unavailable, stale, or invalid, a result from an earlier invocation is never displayed as current.

22. The socket is expected to grant access through a dedicated local group shared by the agent-hosting user and intended MOTD consumers. Documentation and diagnostics do not recommend world-readable socket permissions.

23. Configuring the status-agent socket completely replaces local Docker and Compose status behavior. `go-motd` no longer invokes `docker ps`, searches a Compose directory, or emits separate `Docker Containers` and `Docker Compose` lines.

24. The `compose_dir` setting is removed in this release. A configuration containing it fails with a specific migration error that explains local Compose polling is no longer supported and directs the user to configure `motd-status-agent`; it must not fail only as an unexplained unknown field.

25. Existing configuration unrelated to container status, including media services, tank mount, and network interface, retains its current behavior. Failure or absence of container status never suppresses those sections.

26. The integration is runtime-neutral from the user's perspective. Terminal labels, JSON field names, configuration names, diagnostics, and documentation refer to containers, workloads, or the status agent rather than assuming Podman, Quadlet, Docker, or Compose, except where setup documentation describes the initial agent implementation.

27. The initial status protocol is HTTP/1.1 over the configured Unix socket. `go-motd` requests `GET /v1/status`, accepts a JSON response with numeric `protocol_version` equal to `1`, and rejects other versions. Additive response fields remain compatible within protocol version 1.
