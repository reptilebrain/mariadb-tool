# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a
Changelog](https://keepachangelog.com/en/1.0.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

------------------------------------------------------------------------

## [2.0.0] - 2026-09-12

### Security

- Reconcile every attempted creation stage with independent cleanup timeouts,
  error aggregation, final-state checks, and advisory-lock coordination.
- Protect preexisting resources and suppress potentially secret-bearing SQL errors.
- Escape database-name wildcards in GRANT so privileges target the exact database.
- Add verified remote TLS, system/custom CA trust, and local Unix sockets.
- Enforce private file permissions before writing existing config/CSV/log files;
  reject insecure config permissions on POSIX.
- Open sensitive output files with `O_NOFOLLOW` on Linux/macOS to close the
  final-component symlink substitution race.
- Guarantee all four password character classes with cryptographic shuffling.
- Fail closed on missing/relative XDG or home paths.
- Reject punctuation-only normalized names before they can become valid encoded
  database/user identifiers.

### Changed

- Use collision-free reversible name encoding; reject overlength names rather
  than truncating. Generated names differ from 1.x; review migration guidance.
- Batch returns non-zero on row/read failures and prints outcome counts.
- Explicit CSV export failures now return non-zero without rolling back an
  otherwise successful provisioning; batch reports export failures separately.
- Use separate deadlines for existence checks and each SQL operation.
- Upgrade Go to 1.27.1, mysql to 1.10.1, x/term to 0.46.0, and indirect modules.
- Run integration tests on code PRs/pushes and add race/vulnerability CI checks.
- Pin Actions to verified SHAs, scope release permissions, and verify five
  release archives with SHA256 checksums.

### Testing

- Add regression coverage for ambiguous execution, cleanup/verification failures,
  preexisting resources, lock loss, timeouts, batch summaries, TLS, naming,
  password policy, existing-file permissions, and requested CSV export failures.

------------------------------------------------------------------------

## \[1.5.0\] - 2026-02-22

### Added

-   Docker-based integration test against real MariaDB
-   Opt-in integration test against local MariaDB on `localhost` via DSN
-   Documented test commands for unit, Docker, and localhost integration

### Improved

-   Validation of `-timeout` (must be greater than 0)
-   Clearer error reporting when rollback fails

### Security

-   Rollback now runs in its own context with a separate timeout to reduce partial state risk on timeout
-   Interactive config initialization now hides password input in terminal

------------------------------------------------------------------------

## \[1.4.0\] - 2026-02-19

### Added

-   Full XDG Base Directory compliance
-   Centralized error logging (config, DB connect, runtime errors)
-   Deterministic file placement for config, logs, and CSV export

### Improved

-   All fatal errors are now written to `error.log`
-   Cleaner initialization and DB connection error handling
-   Consistent `0600` permissions on sensitive files

### Security

-   No secrets written to logs
-   Fail-closed behavior preserved under XDG transition

------------------------------------------------------------------------

## \[1.3.0\] - 2026-02-19

### Added

-   Strict normalization of identifiers
-   Wildcard host protection (`%`, `_`) disabled by default
-   Automatic rollback on partial creation failures
-   Extended validation and policy enforcement

### Security

-   Hardened existence checks
-   Prevention of unintended privilege grants

------------------------------------------------------------------------

## \[1.2.0\] - 2026-02-18

### Added

-   Error logging
-   Improved existence checks
-   Updated documentation

------------------------------------------------------------------------

## \[1.0.0\] - 2026-02-18

### Added

-   Initial release
-   MariaDB database + user creation
-   Batch processing support
-   Random secure password generation
