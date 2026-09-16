# Maintaining mariadb-tool

This document describes how to maintain this repository safely and consistently.

## Core Quality Rules

- Keep project-facing text in English (`README`, `CHANGELOG`, release notes, templates).
- Preserve fail-closed behavior and bounded best-effort reconciliation; never claim atomic DDL.
- Never log credentials or other secrets.
- Keep tests and automation green before merging or releasing.

## Branch and PR Flow

- Develop changes in a feature branch.
- Open a pull request to `main` describing changes, validation, simulated failures, and coverage limits. Automated contributors must not merge the PR themselves.
- Ensure CI is green:
  - `go vet ./...`
  - `go test ./... -count=1`
  - `go build ./...`
  - Formatting and the Linux race check
- Prefer squash merge unless preserving commit history is explicitly needed.

## Testing Policy

Run locally before opening PR:

```bash
gofmt -w .
go mod tidy
go vet ./...
go test ./... -count=1
CGO_ENABLED=1 go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Optional local integration against localhost MariaDB:

```bash
MARIADB_TOOL_LOCAL_INTEGRATION=1 \
MARIADB_TOOL_LOCAL_DSN='root:your_password@tcp(127.0.0.1:3306)/' \
go test ./... -run TestProcessDatabaseLocalMariaDBIntegration -count=1 -v
```

Docker integration test:

```bash
MARIADB_TOOL_INTEGRATION=1 \
go test ./... -run TestProcessDatabaseMariaDBIntegration -count=1 -v
```

## GitHub Automation

- `Tests` workflow (`.github/workflows/tests.yml`):
  - Trigger: push to `main`, pull requests targeting `main`, manual dispatch
  - Linux, Windows, macOS: read-only gofmt check, vet, unit tests, build
  - Linux: CGO-enabled race tests with GCC, module checks, govulncheck
  - No secrets or real services; opt-in database integration is explicitly disabled
  - Only contents: read; no documentation path filter, so required checks can run
  - .gitattributes keeps Go files at LF on Windows for the read-only gofmt check
- `Integration` workflow:
  - Trigger: code/module/workflow pushes to main and PRs, nightly, manual
  - Runs Docker-backed MariaDB integration test
- `Release` workflow:
  - Trigger: tag push matching `v*`
  - Builds multi-platform archives and uploads them to GitHub Release

## Release Process

1. Ensure `main` is green in CI.
2. Update `CHANGELOG.md` with a new section using SemVer (`x.y.z`) and current date.
3. Commit and push changelog and related changes.
4. Create an annotated tag:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

5. Verify in GitHub Actions that `Release` workflow succeeds.
6. Verify GitHub Release contains all five expected archives and SHA256SUMS; run `sha256sum -c SHA256SUMS` after download.
7. Publish release notes (can be based on changelog section).

## Hotfix Process

1. Branch from current `main`.
2. Implement minimal, targeted fix.
3. Run tests locally.
4. Open PR and merge after green CI.
5. Tag next patch version (`vX.Y.(Z+1)`).

## Incident and Security Notes

- If any change risks partial state creation, prioritize rollback correctness.
- If a rollback path changes, add or update tests.
- Treat credential handling changes as high-risk; require explicit review.
- Do not include secrets in tests, logs, examples, or issue discussions.


## Security release review

- Recommend a major release (2.0.0) for the normalization migration and stricter
  remote TLS/config permission defaults. Never rename legacy resources implicitly.
- Use trusted private output directories; Windows users must configure NTFS ACLs.
- Coordinate external MariaDB administration: advisory locks protect participating
  tool invocations, not arbitrary clients. Network uncertainty can outlive cleanup.
- Race tests require a C compiler and CGO_ENABLED=1.
- Docker integration requires a running daemon, not just a docker executable.
- Actions are pinned to verified commits. Verify upstream tags before changing pins.
- Only the release publishing job receives contents: write.
- Tests runs even for documentation-only changes. Integration retains path filters;
  do not require that filtered workflow unconditionally without accounting for skips.
- When replacing the old CI workflow, update branch-protection check names to the
  Tests matrix jobs; repository protection settings are not changed by this PR.
- A successful local cross-build does not verify GitHub permissions or actual
  asset publication; verify these on the next authorized release.


## Test isolation and scope

Use `t.TempDir()` for all fixtures and `t.Setenv()` for environment changes;
do not run tests that change environment or standard streams in parallel.
Register database doubles and local TLS servers with `t.Cleanup()`. Do not use
real credentials, home directories, or a developer's running database in unit tests.
Use a regular file blocking a parent directory or a directory as an output target
to simulate portable filesystem failures instead of relying on root/admin-sensitive
permission denial.

Provisioning tests share the in-memory SQL driver, including timeout/EOF,
already-exists, failed DROP, failed verification, and lost-lock scenarios.
TLS tests use an ephemeral local server and test certificates. File tests cover
spaced paths, read failures, dry-run outputs, original input bytes, and CSV
round trips. There is no data-import command; batch-list reads and credential
exports are the applicable file-integrity cases.

Real MariaDB behavior belongs to the separate isolated Docker integration workflow.
Windows mode-bit tests are skipped because NTFS ACL guarantees differ. No unit
test claims to validate production networking, interactive prompts, or ACL setup.

## Coverage badge updates

The README badge is a versioned Linux unit statement-coverage measurement, not
an external rating. The Tests workflow compares it against each Linux run.
When code or tests change the percentage, regenerate it locally on Linux/WSL
using the Go version from go.mod and commit the updated SVG in the same PR:

```bash
MARIADB_TOOL_INTEGRATION=0 MARIADB_TOOL_LOCAL_INTEGRATION=0 \
  go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
bash .github/scripts/coverage-badge.sh coverage.out > .github/badges/coverage.svg
```

CI writes the generated candidate under RUNNER_TEMP and keeps contents: read.
It uploads the Linux profile as the linux-unit-coverage artifact and includes
per-function results in the job summary. To inspect a downloaded profile against
the matching commit, run `go tool cover -html=coverage.out`. Profiles contain
source locations and execution counts, not test credentials.

Platform badges reflect the existing release matrix. Go Report Card was sunset
on July 1, 2026, so the README deliberately uses the live Tests workflow as its
quality-check link instead of adding a broken external rating badge.
