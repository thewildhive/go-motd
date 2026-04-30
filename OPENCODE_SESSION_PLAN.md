# OpenCode Full Project Plan (End-to-End)

This is the complete plan for the `go-motd` modernization and quality pass. It is designed to be dropped directly into a fresh OpenCode session so another agent can continue from context without replaying chat history.

## First 5 Commands To Run

```bash
git status --short --branch
go test ./...
gofmt -l .
go vet ./...
go run . -h
```

Then use this plan section-by-section.

## Scope and Product Decisions (Locked)

1. Breaking change: JSON-only config.
2. Config filenames and paths:
   - `~/.config/motd/config.json`
   - `/opt/motd/config.json`
3. No aliases for legacy keys/services.
4. Remove Organizr support completely.
5. Use Seerr directly for pending requests.
6. Quality bar:
   - No `TODO`/`FIXME` left in changed files.
   - Unit tests for changed logic.
   - `gofmt`, `go vet`, `go test`, build must pass.

## Why This Migration

- Align runtime to Go stdlib-only for config parsing (`encoding/json`, no YAML package).
- Remove incorrect/fragile Organizr request behavior.
- Use direct Seerr endpoint (`/api/v1/request/count`) with `X-Api-Key` for stable pending count.
- Improve parser correctness and reliability in service integrations.

## Research Summary Used for This Plan

### Seerr
- Seerr is the target (replacement path for Overseerr/Jellyseerr deployments).
- Pending count endpoint: `GET /api/v1/request/count`.
- Auth header: `X-Api-Key`.

### Organizr
- Current code path previously used `/api/v2/requests` + `Authorization: Bearer`, which is not aligned with current upstream routes/auth expectations.
- Organizr support is intentionally removed per product decision.

### Other External APIs
- Sonarr/Radarr `wanted/missing` payloads are paged; prefer `totalRecords`.
- Jellyfin sessions parsing should be typed to avoid `map[string]interface{}` fragility.
- `vnstat --json` parsing should match interface and month more defensively.

## Current Implementation Status Snapshot

This section should be validated with `git status` in the new session. Based on latest state:

### Runtime and Config
- JSON loader and strict decode logic added.
- Legacy YAML detection (`config.yml` / `config.yaml`) with explicit migration error.
- Config struct migrated to JSON tags.
- `VERSION` changed to linker-settable variable.

### Service Integrations
- Organizr removed from runtime/config.
- Seerr service integration added.
- Sonarr/Radarr parsing improved (`totalRecords` support).
- Jellyfin parsing made typed.
- vnstat parsing improved via helper functions.

### Tests
- New `main_test.go` covers:
  - JSON decode success/fail/unknown fields
  - legacy YAML detection paths
  - Seerr request count behavior (`httptest`)
  - ARR count parsing
  - Jellyfin session parsing
  - vnstat parsing/month selection
  - version/platform helper behavior

### Artifacts
- Added JSON config files:
  - `config.json`
  - `config.json.sample`
  - `test-default.json`
- Removed YAML config files:
  - `config.yml`
  - `config.yml.sample`
  - `test-default.yml`

### Docs/Installer
- `README.md` rewritten to JSON + Seerr.
- `INSTALL.md` rewritten and de-duplicated.
- `install.sh` updated for JSON sample generation and artifact handling.

## Full Work Plan (Checklist)

### A) Core Runtime Migration
- [x] Replace YAML tags/loaders with strict JSON loader.
- [x] Switch config path references to `config.json` locations.
- [x] Add migration error for detected legacy YAML files.
- [x] Remove Organizr config/service/runtime references.
- [x] Add Seerr service (`/api/v1/request/count`, `X-Api-Key`).
- [x] Make `VERSION` linker-settable for build/release injection.

### B) Reliability and Parsing Correctness
- [x] Eliminate long-lived deferred response closes in service loops by scoping requests.
- [x] Harden Sonarr/Radarr count logic (`totalRecords` fallback).
- [x] Replace dynamic Jellyfin maps with typed decode.
- [x] Improve vnstat interface and month selection logic.

### C) Config Files and Dependency Cleanup
- [x] Remove YAML dependency from `go.mod`/`go.sum`.
- [x] Add JSON config artifacts and remove YAML artifacts.

### D) Tests and Validation
- [x] Add unit tests for all changed parser/helper logic.
- [x] `go test ./...` passing.
- [ ] Run `go test -race ./...` if environment supports CGO compiler toolchain.
  - Note: Windows environment may lack `gcc`; if so, record blocker clearly.

### E) Docs and Installer Alignment
- [x] README updated for JSON + Seerr.
- [x] INSTALL updated and deduplicated.
- [x] Installer sample output switched to JSON schema.
- [x] Remove stale docs references to YAML/Organizr where user-facing.

### F) Release System Hardening (Remaining)
- [ ] Rework `.github/workflows/release.yml` to use one versioning strategy.
- [ ] Remove silent failure masks (`|| true`) in release-critical steps.
- [ ] Remove manual patch bump logic if semantic-release is authoritative.
- [ ] Ensure changelog/tag/release asset steps are coherent and deterministic.
- [ ] Ensure `.releaserc.json` matches intended release flow.
- [ ] Update docs to describe actual release mechanism (not conflicting statements).

## Release Workflow Repair Plan (Detailed)

1. Pick one source of truth:
   - Recommended: semantic-release controls version/tag/changelog/release notes.
2. In workflow:
   - remove manual NEXT_VERSION patch increment block,
   - remove dry-run parsing as a control path,
   - remove `|| true` on commit/push/release steps.
3. Ensure build artifacts use resolved release version once.
4. Ensure changelog generation is real (not dry-run) if changelog updates are expected.
5. Validate branch protection assumptions (no hidden bypass surprises).

## Test Matrix (Must Pass)

1. Valid JSON config loads.
2. Unknown config fields fail with clear error.
3. Missing config prints JSON path guidance.
4. Legacy YAML present prints migration-specific error.
5. Seerr success path renders pending requests.
6. Seerr non-200 path fails gracefully.
7. Sonarr/Radarr count selection uses `totalRecords` correctly.
8. Jellyfin parser handles active/transcode/bitrate variants.
9. vnstat parser selects expected month/interface behavior.
10. Version helpers remain stable.

## Quality Gate Runbook

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

If race test fails due to environment (e.g., missing `gcc` on Windows), document as environment blocker and proceed with explicit note.

Tag hygiene check:

```bash
# Ensure no unresolved markers in touched files
search for TODO|FIXME
```

## Git / Delivery Steps

1. Ensure working tree is clean after changes.
2. Commit in logical units if further edits are made.
3. Push branch to remote.
4. If protected branch requires PR, follow repo policy.

## Risks and Mitigations

- Risk: Release workflow remains inconsistent.
  - Mitigation: prioritize section F before final release cut.
- Risk: Race tests unavailable in local env.
  - Mitigation: run in CI with proper CGO toolchain or document blocker.
- Risk: Hidden docs drift in less visible files.
  - Mitigation: grep audit for YAML/Organizr/version-flow references.

## Definition of Done

All must be true:

1. Runtime behavior matches JSON + Seerr scope.
2. No Organizr runtime/config/docs references (except intentional migration notes/history).
3. Unit tests cover changed logic and pass.
4. Formatting/lint/build checks pass.
5. Release workflow and config no longer conflict.
6. No `TODO`/`FIXME` in changed files.
7. Final handoff includes migration notes and any environment blockers.
