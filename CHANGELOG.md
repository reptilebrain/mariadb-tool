# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a
Changelog](https://keepachangelog.com/en/1.0.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

------------------------------------------------------------------------

## \[1.5.0\] - 2026-02-22

### Added

-   Docker-baserat integrationstest mot riktig MariaDB
-   Opt-in integrationstest mot lokal MariaDB på `localhost` via DSN
-   Dokumenterade testkommandon för unit-, Docker- och localhost-integration

### Improved

-   Validering av `-timeout` (måste vara större än 0)
-   Tydligare felrapportering om rollback misslyckas

### Security

-   Rollback körs i eget context med separat timeout för att undvika partiellt state vid timeout
-   Interaktiv lösenordsinmatning i config-init är nu dold i terminalen

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
