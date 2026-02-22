# Maintaining mariadb-tool

This document describes how to maintain this repository safely and consistently.

## Core Quality Rules

- Keep project-facing text in English (`README`, `CHANGELOG`, release notes, templates).
- Preserve fail-closed behavior and no-partial-state guarantees.
- Never log credentials or other secrets.
- Keep tests and automation green before merging or releasing.

## Branch and PR Flow

- Develop changes in a feature branch.
- Open a pull request to `main`.
- Ensure CI is green:
  - `go vet ./...`
  - `go test ./... -count=1`
- Prefer squash merge unless preserving commit history is explicitly needed.

## Testing Policy

Run locally before opening PR:

```bash
go test ./... -count=1
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

- `CI` workflow:
  - Trigger: push to `main`, pull requests
  - Runs `go vet` and unit tests
- `Integration` workflow:
  - Trigger: nightly schedule and manual dispatch
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
6. Verify GitHub Release contains expected archives.
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
