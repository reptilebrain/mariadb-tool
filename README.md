# MariaDB User & Database Creator

A lightweight Go utility to automate the creation of MariaDB databases and users with random secure passwords.

## Features

- **Strict Safety:** Checks if **both** the user and the database already exist before execution to prevent data loss or password overwriting.
- **Improved Syntax:** Uses SQL backticks to support complex database names (e.g., `my.domain.com`).
- **Secure Passwords:** Generates 16-character cryptographically secure passwords using `crypto/rand`.
- **Flexible Input:** Supports single commands or bulk batch processing via text files.
- **Automatic Logging:** - Successful credentials are saved to `accounts.csv`.
  - Errors and skips are logged to `error.log`.
- **Easy Config:** Generates a template `config.ini` on the first run.

## Installation

1. **Initialize and download dependencies:**
   ```bash
   go mod tidy
   ```
2. **Usage:**
   ```
   db-tool -c <name> Create single database/user
   db-tool -f <file.txt> Batch processing from file
   ```

## Disclaimer

**Use at your own risk.** This tool is provided "as is" without any warranty. The authors are not responsible for any data loss, security breaches, or service interruptions caused by the use of this software. Always test in a staging environment before running against a production database.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
