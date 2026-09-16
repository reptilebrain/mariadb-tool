# MariaDB User & Database Creator

[![Tests](https://github.com/reptilebrain/mariadb-tool/actions/workflows/tests.yml/badge.svg)](https://github.com/reptilebrain/mariadb-tool/actions/workflows/tests.yml)
[![Integration](https://github.com/reptilebrain/mariadb-tool/actions/workflows/integration.yml/badge.svg)](https://github.com/reptilebrain/mariadb-tool/actions/workflows/integration.yml)
[![Release Workflow](https://github.com/reptilebrain/mariadb-tool/actions/workflows/release.yml/badge.svg)](https://github.com/reptilebrain/mariadb-tool/actions/workflows/release.yml)
[![Release](https://img.shields.io/github/v/tag/reptilebrain/mariadb-tool?sort=semver)](https://github.com/reptilebrain/mariadb-tool/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/reptilebrain/mariadb-tool)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

See [CHANGELOG.md](CHANGELOG.md) for version history.

A security-focused Go CLI tool for safely creating paired MariaDB
databases and users.

This tool is designed with a **fail-closed philosophy**:\
If anything unexpected exists, nothing is modified.

------------------------------------------------------------------------

## Design Principles

-   **Fail Closed** --- If either the database or user already exists,
    creation is aborted.
-   **Idempotent** --- Safe to run repeatedly.
-   **Deterministic Naming** --- Domain inputs are normalized into valid
    identifiers.
-   **Best-effort Reconciliation** --- Failed creation triggers bounded cleanup
    and independent verification; unresolved state is reported.
-   **No Secrets in Logs** --- Passwords are never written to logs.

------------------------------------------------------------------------

## Features

### Safety & Policy

-   Separate checks for database and user existence
-   Strict input validation
-   Optional domain name normalization (enabled by default)
-   Wildcard host (`%`, `_`) disabled by default
-   Reconciliation after any attempted CREATE or GRANT fails
-   Timeout protection for DB operations
-   XDG-compliant configuration and state handling

------------------------------------------------------------------------

## Naming & Normalization

With `-normalize` (enabled by default), names use reversible escaping:

```text
example.com     -> example_dcom
foo-bar.com     -> foo_hbar_dcom
foo.bar.com     -> foo_dbar_dcom
foo_dbar.com    -> foo_udbar_dcom
ABC             -> _ca_cb_cc
```

`.` becomes `_d`, `-` becomes `_h`, `_` becomes `_u`, and uppercase
letters become `_c` plus the lowercase letter. Whitespace around input is
trimmed. Invalid input is rejected. Raw normalized input must contain at least
one ASCII letter or digit, so punctuation-only values such as `.`, `---`, or
`._-` are rejected instead of becoming real database/user names. Encoded names
longer than 64 characters are rejected, never truncated or hashed. This prevents
normalization collisions, including case differences on case-insensitive servers.

**Migration:** names differ from 1.x. Review `-dry-run` output before upgrading
automation. Existing names are never renamed, adopted, or modified. To refer to
an exact legacy identifier, use `-normalize=false -c example_com`; existing
resources are skipped. Switching normalization modes can refer to the same
identifier, but existing-resource checks still prevent modifications.

------------------------------------------------------------------------

## Passwords

-   20 characters
-   At least one lowercase, uppercase, digit, and symbol from `!#%&`
-   Generated and shuffled using `crypto/rand`
-   Safe for SQL literals; quote credentials when using them in a shell
-   Successful credentials are printed to stdout; protect terminal/session output

------------------------------------------------------------------------

## Execution Modes

-   Single creation (`-c`)
-   Batch mode (`-f`)
-   Dry-run mode (`-dry-run`)
-   Config initialization (`-i`)
-   Optional credential export (`-export-csv`)

------------------------------------------------------------------------

## Installation

Requires Go 1.27.1 or newer.

``` bash
go build -o mariadb-tool
```

------------------------------------------------------------------------

## Usage

Initialize configuration:

``` bash
./mariadb-tool -i
```

Create a single database/user:

``` bash
./mariadb-tool -c example.com
```

Dry run:

``` bash
./mariadb-tool -dry-run -c example.com
```

Batch processing:

``` bash
./mariadb-tool -f list.txt
```

Allow wildcard host (explicit opt-in):

``` bash
./mariadb-tool -allow-wildcard-host -user-host "%" -c example.com
```

------------------------------------------------------------------------

## XDG File Locations (Default)

The tool follows the XDG Base Directory Specification.

Unless overridden via flags:

**Config**

    ~/.config/mariadb-tool/config.ini

**Logs**

    ~/.local/state/mariadb-tool/error.log

**CSV Export**

    ~/.local/share/mariadb-tool/accounts.csv

No files are written to the current working directory unless explicitly
specified. XDG/home paths must be absolute. If defaults cannot be resolved,
execution fails unless `-config`, `-error-log` and (when exporting) `-csv` are
explicitly provided.

------------------------------------------------------------------------

## Batch File Format

Plain text, one entry per line:

``` text
example.com
shop.example.com
test-site.io
```

Comments (`#` or `;`) and blank lines are ignored.
Processing continues after row errors. The summary counts Created, Skipped,
and Failed; dry runs have a separate Dry-run count. When `-export-csv` is used,
credential-export failures are counted separately. Any failed row, input read
error, or requested credential-export failure results in a non-zero process
exit status. A skip is not a failure.

```text
Batch complete:
Created: 8
Skipped: 2
Failed: 1
Export failed: 1
```

A CSV export failure happens after successful provisioning, so the created
resources are not rolled back. Credentials are still printed to stdout, the
resource remains counted as Created, and the process returns non-zero so
automation cannot silently treat the missing credential file as success.

------------------------------------------------------------------------

## Configuration

`config.ini`:

``` ini
[mariadb]
username=admin
password=your_secure_password
hostname=localhost
port=3306
tls=auto
```

On POSIX, the file is enforced to `0600`, including when it already exists.
Config files with group/other permissions are rejected before parsing;
fix them with `chmod 600 /path/to/config.ini`. CSV and log files are tightened
before appending. Symlink output files are rejected. On Linux and macOS,
sensitive output files are opened with `O_NOFOLLOW`, closing the final-component
symlink substitution race between checking a path and opening it. Use trusted
parent directories. Windows requires suitable account-only NTFS ACLs: Go chmod
does not enforce POSIX confidentiality there.

------------------------------------------------------------------------

## TLS and local sockets

`tls=auto` is the default: remote TCP uses verified TLS (TLS 1.2 minimum,
system CA roots and hostname verification), while `localhost` and literal
loopback IPs retain plaintext TCP compatibility. There is no TLS downgrade.
Use `tls=true` to require verified TLS even on loopback.

```ini
[mariadb]
username=admin
password=your_secure_password
hostname=db.example.com
port=3306
tls=true
tls-ca=/absolute/path/private-ca.pem
```

`tls-ca` is optional and adds a PEM CA to system trust. CLI overrides are
`-tls auto|true|false`, `-tls-ca /path/ca.pem`, and `-socket /path/mysql.sock`.
A CA file enables TLS in auto mode even on loopback. Invalid CAs and conflicting
options fail closed. `skip-verify` and opportunistic TLS are unsupported.
**Remote TCP without TLS is unsafe** and exposes credentials/data; explicit
`tls=false` permits it with a warning. For tunnels/proxies, use verified TLS
when the end-to-end path is not trusted.

For local MariaDB, `socket=/run/mysqld/mysqld.sock` replaces hostname/port.
It must be absolute; explicit TLS/CA options cannot be combined with a socket.

## Timeouts and reconciliation

`-timeout` (default 6s) applies separately to each existence query,
CREATE DATABASE, CREATE USER, GRANT, and initial connection. Cleanup uses
fresh background contexts: 3s for each DROP, lock check, and verification query.
Total execution time can therefore exceed `-timeout`.

Before creating anything, both database and exact user/host must be absent.
The admin account needs SELECT access to `mysql.user`; insufficient access
fails closed. A separate connection holds a MariaDB advisory lock for the
database name while SQL operations run on other connections.
Allow at least two server connections per invocation.

On error, every relevant DROP is attempted independently, including after a
lost response, followed by independent checks for both resources. Multiple
errors are combined. An explicit already-exists error protects that conflicting
resource. If lock ownership is lost, destructive cleanup is withheld and
remaining/unknown state is reported.

This is robust **best-effort reconciliation**, not atomic DDL or an absolute
no-partial-state guarantee. Server/network outages, delayed server execution,
process termination, and external administrators can leave resources behind.
The advisory lock coordinates this tool only. Do not run other clients that
create, replace, or drop the same resources concurrently: MariaDB offers no
transactional ownership token for this DDL, so external replacement between
checks and cleanup cannot be ruled out. Inspect reported resources manually
before deleting anything. No existing resource is automatically repaired.

------------------------------------------------------------------------

## Logging

### error.log

Logs:

-   Validation failures
-   SQL error codes/categories (server text is suppressed to protect secrets)
-   Batch line numbers
-   Configuration or connection errors

Passwords are never logged.

------------------------------------------------------------------------

### accounts.csv (optional)

Credentials are exported only when `-export-csv` is used.

Format:

  -----------------------------------------------------------------------
  Timestamp            Database      Username      Password
  -------------------- ------------- ------------- ----------------------
  2026-02-19 14:27     example_com   example_com   3oJb39NT90YaAx1c&wI6

  -----------------------------------------------------------------------

Existing CSV and log files are also enforced to `0600` before writing on POSIX.
If an explicitly requested CSV export cannot be written after provisioning, the
credentials remain on stdout and the command exits non-zero without rolling back
the successfully created database/user.

------------------------------------------------------------------------

## Security Model

This tool:

-   Does not overwrite existing users
-   Does not modify existing databases
-   Does not escalate privileges
-   Does not allow wildcard hosts unless explicitly enabled
-   Attempts bounded cleanup and verifies state after creation failures
-   Does not leak credentials to logs

It is intended for administrative automation, not multi-tenant
self-service.

------------------------------------------------------------------------

## Testing

The default unit suite uses a fake SQL driver and local `httptest` TLS servers.
Files use `t.TempDir()`, and environment overrides use `t.Setenv()` without
parallel environment-mutating tests. No database, Docker, credentials, or user
configuration is needed. The Tests workflow explicitly disables both opt-in
database integration modes.

Coverage includes configuration and connection errors, dry run without DDL or
output writes, preexisting names and creation races, paths containing spaces,
unchanged config/batch input bytes, append-only CSV integrity, batch read/export
failures, TLS verification, passwords, and reconciliation failures.
Database-specific integration remains a separate Docker workflow.

Unit tests do not establish real MariaDB DDL/privilege semantics, production
network behavior, interactive terminal handling, or Windows ACL confidentiality.
POSIX permission/symlink assertions are skipped on Windows. Dry-run assertions
exercise provisioning and batch functions; first-run CLI config initialization
and interactive prompts are outside this coverage.

## Automation

-   **Tests** (`.github/workflows/tests.yml`) runs on PRs targeting `main`, pushes to `main`, and manual dispatch. Linux, Windows, and macOS each check formatting without rewriting files, run `go vet`, unit tests, and `go build`. Linux also runs race tests, module metadata checks, and `govulncheck`. Permissions are limited to `contents: read`.
-   **Integration** (`.github/workflows/integration.yml`) runs on relevant main pushes and PRs, nightly, and manually. Code/module/workflow path filters avoid database tests for documentation-only changes.
-   **Release** (`.github/workflows/release.yml`) runs on tag pushes (`v*`) and publishes Linux/macOS amd64/arm64 and Windows amd64 archives plus SHA256SUMS. All five archives must exist; only the publishing job has contents write permission.

## Project Operations

-   Issue templates are available for bug reports and feature requests (`.github/ISSUE_TEMPLATE/`).
-   Pull requests use a default review checklist (`.github/pull_request_template.md`).
-   Maintainer process and release operations are documented in [`MAINTAINING.md`](MAINTAINING.md).

Run unit tests:

``` bash
go test ./...
```

Run integration test (requires Docker):

``` bash
MARIADB_TOOL_INTEGRATION=1 go test ./... -run TestProcessDatabaseMariaDBIntegration -count=1
```

Run integration test against local MariaDB (`localhost`):

``` bash
MARIADB_TOOL_LOCAL_INTEGRATION=1 \
MARIADB_TOOL_LOCAL_DSN='root:your_password@tcp(127.0.0.1:3306)/' \
go test ./... -run TestProcessDatabaseLocalMariaDBIntegration -count=1
```

Scenarios verified:

-   Normal creation
-   Existing DB
-   Existing user
-   Privilege failure with automatic rollback
-   Invalid input rejection
-   Wildcard host enforcement
-   XDG-compliant file placement

------------------------------------------------------------------------

## License

GNU General Public License v3.0 or later (GPL-3.0-or-later).

See the [LICENSE](LICENSE) file for details.

## Disclaimer

Provided as-is without warranty.\
Always test against a staging environment before production use.
