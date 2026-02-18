# MariaDB User & Database Creator

A lightweight Go utility to automate the creation of MariaDB databases and users with random secure passwords.

## Features

- **Strict Safety:** Checks if **both** the user and the database already exist before execution to prevent data loss or password overwriting.
- **Improved Syntax:** Uses SQL backticks to support complex database names (e.g., `my.domain.com`).
- **Secure Passwords:** Generates 16-character secure passwords.
- **Flexible Input:** Supports single commands or bulk batch processing via text files.
- **Automatic Logging:** - Successful credentials are saved to `accounts.csv`.
  - Errors and skips are logged to `error.log`.
- **Easy Config:** Generates a template `config.ini` on the first run or via the `-i` flag.

## Installation & Usage

1. **Build the binary:**

    ```bash
    go build -o db-tool
    ```

2. **Usage:**

    ```bash
    ./db-tool -i              # Initialize configuration
    ./db-tool -c <name>       # Create single database/user
    ./db-tool -f <file.txt>   # Batch processing from file
    ```

## Output & Logging

### Accounts.csv

Every successful creation is logged to `accounts.csv`. This file is automatically created with headers if it doesn't exist. It uses the following format:

| Timestamp | Database | Username | Password |
| :--- | :--- | :--- | :--- |
| 2026-02-18 20:15 | my_db | my_db | aB1#cDe2fG3!hI4j |

### Error log

```bash
[2026-02-18 20:19:11] Skipping 'yourdomain.com': database or user already exists
```

## Configuration

The tool uses a config.ini file for database credentials:

```ini
    [mariadb]
    username=admin
    password=your_secure_password
    hostname=localhost
    port=3306
```

## Contributing

This project is feature-complete for my needs. If you'd like to add new features or make changes, feel free to fork the repository!

## Disclaimer

Use at your own risk. This tool is provided "as is" without any warranty. The authors are not responsible for any data loss, security breaches, or service interruptions caused by the use of this software. Always test in a staging environment before running against a production database.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
