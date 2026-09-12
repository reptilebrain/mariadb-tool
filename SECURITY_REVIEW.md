# Security and quality review

Review date: 2026-09-12
Working branch: codex/security-quality-review
Target release: **2.0.0**

## Implemented changes

- Reconcile failures after CREATE DATABASE, CREATE USER, and GRANT, including
  ambiguous timeout/network outcomes. Independently attempt relevant cleanup
  steps, verify both resources, and join multiple errors.
- Protect resources found before creation and resources that trigger explicit
  already-exists errors. Coordinate participating invocations with a separate
  advisory-lock connection; discard uncertain lock connections.
- Apply fresh deadlines to individual existence queries and SQL operations.
  Cleanup and its verification use independent 3-second deadlines.
- Add verified remote TLS by default, system/custom CA trust, explicit plaintext
  opt-out, and local Unix socket support. Preserve localhost TCP compatibility.
- Upgrade Go to 1.27.1, mysql to 1.10.1, x/term to 0.46.0, x/sys to 0.48.0,
  and edwards25519 to 1.2.0.
- Continue batch processing after errors, log row errors, print outcome counts,
  and return non-zero for row/read failures.
- Enforce 0600 before writing existing private files on POSIX and reject
  group/other-readable config files. Reject symlink output targets.
- Guarantee all four password character classes and cryptographically shuffle.
- Replace lossy normalization with reversible escaping and reject long names.
- Fail closed if default XDG/home paths cannot be safely resolved.
- Suppress SQL/driver text that might reveal passwords.
- Escape underscores in GRANT database patterns to prevent overbroad privileges.
- Run integration CI on relevant PRs/pushes as well as nightly/manual triggers.
  Add race, vulnerability, format and module checks; pin Actions to verified SHAs.
- Limit contents: write to release publishing; require all five build archives,
  generate SHA256SUMS, and fail publishing on unmatched asset paths.
- Update README, CHANGELOG, and MAINTAINING with behavior and migration guidance.

## Validation results

| Check | Result |
| --- | --- |
| gofmt -w . | Passed |
| go mod tidy | Passed |
| go vet ./... | Passed |
| go test ./... | Passed (opt-in database integration tests skipped) |
| CGO_ENABLED=1 go test -race ./... | Passed |
| go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./... | No vulnerabilities found |
| actionlint v1.7.12 | Passed |
| git diff --check | Passed |
| Actual release build script: Linux amd64/arm64, macOS amd64/arm64, Windows amd64 | All five final builds passed |
| Actual release verification script and SHA256 checks | Passed |
| Release verification with empty artifacts | Correctly failed |
| Docker MariaDB integration (explicitly attempted) | Blocked before server startup: Docker WSL integration unavailable; Windows Docker daemon also unavailable |
| GitHub Release upload | Not executed locally; workflow wiring validated, requires a real authorized release run |

GCC and libc development headers were installed in TestUbuntu to enable the
requested race detector. Build outputs are in the ignored dist/ directory.
No release tag or GitHub Release was created during this review.

Regression coverage includes each failed SQL stage with applied/unapplied
side effects, timeout/network-like failures, both DROP failures, verification
failures, independent deadlines, preexisting resources, already-exists races,
lock loss and uncertain acquisition, case-equivalent locks, batch continuation,
reversible naming, password classes, existing 0644 files, fail-closed XDG paths,
and real TLS handshakes accepting a trusted CA while rejecting unknown CAs and
wrong hostnames. The Docker test now also checks existing resources, lock
conflicts, and exact grant patterns, but those server checks remain unverified
in this environment.

## Remaining limitations and migration

- Reconciliation is best effort, not transactional DDL. Delayed server execution,
  network outages, process death, and external clients can still leave partial
  state. Verify remaining resources manually.
- Advisory locks coordinate participating tool instances only. External clients
  must not concurrently create/replace/drop the same resources; MariaDB does
  not provide transactional ownership tokens for these DDL operations.
- The admin account needs SELECT access to mysql.user and at least two available
  server connections. Inability to inspect accounts fails closed.
- Names differ from 1.x. Review dry-run output and use -normalize=false for exact
  legacy identifiers. Nothing is automatically renamed or repaired.
- Existing remote plaintext/self-signed configurations may require explicit TLS
  configuration. Existing insecure POSIX config files require chmod 600.
- Windows requires private NTFS ACLs; POSIX chmod does not guarantee Windows
  confidentiality. Output parent directories must be trusted.
- Credentials are intentionally printed to stdout. CSV-export errors retain
  existing warning semantics after successful creation and count as Created.
- Raw server errors are suppressed, which reduces diagnostic detail.
- Documentation-only path filters can interact with required GitHub checks.
  Check repository branch protection when enabling these workflows.
- MariaDB integration and actual GitHub asset publication must be verified before
  releasing. **2.0.0** is recommended because normalization changes generated
  identifiers and TLS/config validation is intentionally stricter.

## Changed files

- .github/workflows/ci.yml
- .github/workflows/integration.yml
- .github/workflows/release.yml
- CHANGELOG.md
- MAINTAINING.md
- README.md
- SECURITY_REVIEW.md
- config.go
- config_test.go
- connection.go
- connection_test.go
- db.go
- db_integration_test.go
- db_test.go
- go.mod
- go.sum
- io.go
- main.go
- password.go
- password_test.go
- reconcile.go
- validate.go
- validate_test.go

## References checked

- [Go release downloads](https://go.dev/dl/?mode=json)
- [MySQL Go driver](https://github.com/go-sql-driver/mysql)
- [MariaDB GRANT semantics](https://mariadb.com/docs/server/reference/sql-statements/account-management-sql-statements/grant)
- Action commit pins were checked against the corresponding upstream Git tags.
