# OpenCode Quick Plan

Use this for fast handoff execution. Full context is in `OPENCODE_SESSION_PLAN.md`.

## Clean-Slate Rule

Treat all tasks as unverified until re-checked in this session.
Do not trust prior checkmarks; verify with commands/tests and evidence before marking done.

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

## Entire Todo List (Snapshot)

`[ ]=not yet verified`, `[-]=blocked`

- [ ] lock scope and quality bar
- [ ] baseline behavior capture
- [ ] strict JSON config migration
- [ ] legacy YAML migration errors
- [ ] config path switch to `config.json`
- [ ] remove YAML module dependency
- [ ] remove Organizr support entirely
- [ ] add Seerr pending-request integration
- [ ] improve Sonarr/Radarr/Jellyfin/vnstat parsing reliability
- [ ] make `VERSION` linker-settable
- [ ] update installer for JSON schema
- [ ] rewrite/dedupe docs (README/INSTALL)
- [ ] add broad unit test coverage in `main_test.go`
- [ ] run `gofmt`, `go vet`, `go test`, `go build`
- [-] run `go test -race ./...` (blocked locally: missing `gcc`/CGO toolchain)
- [ ] reconcile and harden release pipeline (`.github/workflows/release.yml`, `.releaserc.json`)
- [ ] final docs consistency sweep across remaining files
- [ ] final no-`TODO`/`FIXME` scan after last edits
- [ ] final scenario validation + handoff report

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
go build -buildvcs=false -o bin/motd .
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
