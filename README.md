# MariaDB User & Database Creator

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
-   **No Partial State** --- If creation fails mid-process, cleanup is
    automatically performed.
-   **No Secrets in Logs** --- Passwords are never written to logs.

------------------------------------------------------------------------

## Features

### Safety & Policy

-   Separate checks for database and user existence
-   Strict input validation
-   Optional domain name normalization (enabled by default)
-   Wildcard host (`%`, `_`) disabled by default
-   Automatic rollback if `CREATE USER` or `GRANT` fails
-   Timeout protection for DB operations
-   XDG-compliant configuration and state handling

------------------------------------------------------------------------

## Naming & Normalization

With `-normalize` (enabled by default):

    example.com        → example_com
    shop.example.io    → shop_example_io

Invalid inputs (e.g. `invalid name!!`) are rejected.

Maximum identifier length is 64 characters.\
Long names are truncated with a short hash suffix.

------------------------------------------------------------------------

## Passwords

-   20 characters
-   Alphanumeric + selected symbols
-   Safe for SQL literals

------------------------------------------------------------------------

## Execution Modes

-   Single creation (`-c`)
-   Batch mode (`-f`)
-   Dry-run mode (`-dry-run`)
-   Config initialization (`-i`)
-   Optional credential export (`-export-csv`)

------------------------------------------------------------------------

## Installation

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
specified.

------------------------------------------------------------------------

## Batch File Format

Plain text, one entry per line:

``` text
example.com
shop.example.com
test-site.io
```

Comments (`#` or `;`) and blank lines are ignored.

------------------------------------------------------------------------

## Configuration

`config.ini`:

``` ini
[mariadb]
username=admin
password=your_secure_password
hostname=localhost
port=3306
```

The file is created with `0600` permissions.

------------------------------------------------------------------------

## Logging

### error.log

Logs:

-   Validation failures
-   SQL errors
-   Skipped operations
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

The file is created with `0600` permissions.

------------------------------------------------------------------------

## Security Model

This tool:

-   Does not overwrite existing users
-   Does not modify existing databases
-   Does not escalate privileges
-   Does not allow wildcard hosts unless explicitly enabled
-   Cleans up partially created resources on failure
-   Does not leak credentials to logs

It is intended for administrative automation, not multi-tenant
self-service.

------------------------------------------------------------------------

## Testing

Tested against MariaDB using isolated Docker environments.

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
