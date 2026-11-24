All commits and releases must go through a pull request against protected `main`; never push directly to `main` or bypass required checks.

- Work on a short-lived branch. Use a Conventional Commit PR title and wait for `CI / Required` before merging.
- Use the tools pinned in `mise.toml`: `mise trust mise.toml`, then `mise install`. Use `make build` for the CLI and `make check` for static checks and all supported build targets.
- Prove changes with the real CLI. Include the command and relevant output in the PR; capture before and after for terminal layout changes. Redact credentials. Do not add or run automated test suites.
- Keep runtime dependencies in the Go standard library. Preserve machine-readable JSON when changing terminal presentation.
- For a requested release, merge the generated Release Please PR after its checks pass. Verify the signed GitHub release assets and the APT deployment before calling it shipped. Do not tag or publish manually.
