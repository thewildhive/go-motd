# OpenCode Session Plan (JSON + Seerr Migration)

Use this file as the single source of truth for continuing this migration in a new OpenCode session.

## First 5 Commands To Run

Run these first in a new session to rehydrate context fast:

```bash
git status --short
go test ./...
gofmt -l .
go vet ./...
go run . -h
```

Then continue with the remaining checklist in this file.

## Objective

Deliver a high-quality breaking-change release of `go-motd` with:
- JSON-only config (`config.json` paths)
- full removal of Organizr support
- direct Seerr integration for pending requests
- robust tests and strict quality gates

## Non-Negotiable Constraints

1. No config aliases (`overseerr` alias is not allowed).
2. No Organizr compatibility mode; remove it completely.
3. No `TODO`/`FIXME` left in changed files.
4. Unit tests for all changed logic paths.
5. Final validation must include `gofmt`, `go vet`, `go test`, and build.

## Current Working Tree Snapshot

Modified/added so far:
- `main.go`
- `main_test.go` (new)
- `go.mod`
- `go.sum`
- `install.sh`
- `README.md`
- `INSTALL.md`
- `AGENT.md`
- `config.json` (new)
- `config.json.sample` (new)
- `test-default.json` (new)
- removed: `config.yml`, `config.yml.sample`, `test-default.yml`

## Scope Checklist

### A) Core Runtime Migration
- [x] Convert config tags/loader from YAML to strict JSON (`encoding/json`, unknown fields rejected).
- [x] Switch config paths to:
  - `~/.config/motd/config.json`
  - `/opt/motd/config.json`
- [x] Detect legacy YAML (`config.yml`/`config.yaml`) and exit with explicit migration error.
- [x] Remove Organizr runtime/config references.
- [x] Add Seerr runtime integration (`GET /api/v1/request/count`, `X-Api-Key`).
- [x] Make `VERSION` linker-settable (`var VERSION = ...`).

### B) Reliability/Correctness Improvements
- [x] Fix deferred response-body handling in service loops by scoping requests per instance.
- [x] Harden Sonarr/Radarr counts (`totalRecords` fallback to `len(records)`).
- [x] Harden Jellyfin parsing with typed structures (no fake `0.00 Mbps`).
- [x] Improve vnstat parsing/interface selection with helper functions.

### C) Config + Installer + Docs
- [x] Replace default/sample/test config artifacts with JSON versions.
- [x] Installer writes valid `config.json` matching current schema.
- [x] README rewritten for JSON + Seerr.
- [x] INSTALL rewritten/deduplicated.

### D) Tests
- [x] Add unit tests for JSON decode and migration behavior.
- [x] Add unit tests for Seerr request decoding via `httptest`.
- [x] Add unit tests for ARR count parsing.
- [x] Add unit tests for Jellyfin parsing logic.
- [x] Add unit tests for vnstat month/interface parsing.
- [x] Add unit tests for version compare/platform asset name behavior.

### E) Still Required Before Finalize
- [ ] Reconcile release workflow and semantic-release config drift:
  - `.github/workflows/release.yml`
  - `.releaserc.json`
  - ensure no `|| true` on critical release steps
  - remove conflicting manual version-bump logic vs semantic-release behavior
- [ ] Validate documentation consistency across all remaining docs (`CONTRIBUTING.md`, `QUICK_START.sh`, etc.).
- [ ] Run final quality gates and scenario checks (see Runbook below).

## Release Workflow Repair Plan

1. Choose a single release source of truth:
   - Recommended: semantic-release controls version/tag/changelog/release.
2. Remove dry-run + manual patch increment logic from workflow.
3. Remove silent success masking (`|| true`) from release-critical commands.
4. Ensure asset build uses resolved version consistently.
5. Ensure docs reflect actual release behavior (no contradictory statements).

## Test Matrix (Must Pass)

1. JSON config valid -> app loads.
2. JSON config unknown field -> parse fails clearly.
3. No config file -> missing-config error with JSON paths.
4. Legacy YAML exists -> migration-specific error.
5. Seerr success path -> pending count renders.
6. Seerr non-200 path -> graceful skip + debug log.
7. Sonarr/Radarr counts from `totalRecords`.
8. Jellyfin active/transcode parsing with/without bitrate.
9. vnstat parser picks current/latest month and interface.

## Quality Gate Runbook

Run in repo root:

```bash
gofmt -l .
go vet ./...
go test ./...
go build -o bin/motd main.go
```

Race tests:

```bash
go test -race ./...
```

Note: on Windows, `-race` requires CGO and a C compiler in PATH (e.g. `gcc`). If unavailable, mark as environment blocker and record explicitly in final handoff.

Tag scan before final response:

```bash
# ensure no unresolved markers in changed files
grep for TODO|FIXME (project-wide or changed files)
```

## Definition of Done

All items below must be true:
- Runtime behavior matches JSON+Seerr scope.
- No Organizr support remains.
- Unit tests cover all touched logic and pass.
- Formatting/lint/build pass.
- Release workflow no longer has conflicting version logic or silent critical failures.
- No `TODO`/`FIXME` left in changed files.
- Final summary includes migration notes and any environment blockers.
