# OpenCode Quick Plan

Use this for fast handoff execution. Full context is in `OPENCODE_SESSION_PLAN.md`.

## Bootstrap (run first)

```bash
git status --short --branch
go test ./...
gofmt -l .
go vet ./...
go run . -h
```

## Locked Decisions

- JSON-only config (`config.json` paths)
- remove Organizr completely
- use Seerr directly (`/api/v1/request/count`, `X-Api-Key`)
- no aliases
- no `TODO`/`FIXME` in changed files

## What Is Already Done

- Runtime migrated to strict JSON config loading
- legacy YAML detection and migration error messaging
- Seerr integration added
- Organizr removed from runtime/config
- parser hardening for Jellyfin/Sonarr/Radarr/vnstat
- tests added in `main_test.go`
- JSON config artifacts added (`config.json`, `config.json.sample`, `test-default.json`)
- README/INSTALL rewritten
- installer updated for JSON sample generation

## Remaining Work

1. Reconcile release system:
   - fix `.github/workflows/release.yml`
   - align `.releaserc.json`
   - remove `|| true` in release-critical steps
   - remove conflicting manual version bump logic
2. Final doc consistency sweep (`CONTRIBUTING.md`, `QUICK_START.sh`, others).
3. Final quality gate run and report.

## Final Quality Gates

```bash
gofmt -l .
go vet ./...
go test ./...
go build -o bin/motd main.go
```

Race test when environment supports CGO compiler:

```bash
go test -race ./...
```

## Completion Criteria

- tests and static checks pass
- release flow no longer conflicts
- no unresolved `TODO`/`FIXME` in changed files
- docs and behavior are aligned
