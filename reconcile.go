package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Do not expose server/driver messages: they may quote SQL containing a password.
// Preserve useful machine-readable categories without retaining unsafe text.
func safeDBError(err error) error {
	if err == nil {
		return nil
	}
	var my *mysql.MySQLError
	if errors.As(err, &my) {
		return fmt.Errorf("MariaDB error %d (server message suppressed)", my.Number)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, io.EOF) {
		return io.EOF
	}
	if errors.Is(err, driver.ErrBadConn) {
		return driver.ErrBadConn
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("database network timeout")
	}
	return errors.New("database operation failed (details suppressed to protect credentials)")
}
func mysqlErrorNumber(err error) uint16 {
	var my *mysql.MySQLError
	if errors.As(err, &my) {
		return my.Number
	}
	return 0
}
func execTimedSQL(db *sql.DB, timeout time.Duration, query string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return execSQL(ctx, db, query)
}
func databaseExistsTimed(db *sql.DB, name string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var value string
	err := db.QueryRowContext(ctx, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func userExistsTimed(db *sql.DB, name, host string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return userExists(ctx, db, name, host)
}

// Keep the lock on a different connection so an operation timeout does not
// release it. All tool invocations targeting the same database share this lock.
// Other administrative clients must coordinate separately.
type nameLock struct {
	conn *sql.Conn
	key  string
}

func acquireNameLock(db *sql.DB, name string, timeout time.Duration) (*nameLock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, safeDBError(err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte("mariadb-tool:"+strings.ToLower(name))))
	var acquired sql.NullInt64
	err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", key).Scan(&acquired)
	if err != nil || !acquired.Valid || acquired.Int64 != 1 {
		// An uncertain GET_LOCK response may still have acquired the lock.
		// Discard the connection instead of returning a potentially locked session.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		conn.Close()
		if err != nil {
			return nil, safeDBError(err)
		}
		return nil, errors.New("another operation holds the database name lock")
	}
	return &nameLock{conn, key}, nil
}
func (l *nameLock) check() error {
	ctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()
	var owned sql.NullBool
	if err := l.conn.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?) = CONNECTION_ID()", l.key).Scan(&owned); err != nil {
		return safeDBError(err)
	}
	if !owned.Valid || !owned.Bool {
		return errors.New("database name lock is no longer held")
	}
	return nil
}
func (l *nameLock) close() {
	ctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()
	var released sql.NullInt64
	err := l.conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", l.key).Scan(&released)
	if err != nil || !released.Valid || released.Int64 != 1 {
		// A pooled connection must never retain the advisory lock.
		_ = l.conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = l.conn.Close()
}

func reconcile(db *sql.DB, lock *nameLock, name, host string, dropDB, dropUser bool) error {
	var errs []error
	cleanup := func(label, query string) {
		if err := lock.check(); err != nil {
			errs = append(errs, fmt.Errorf("%s withheld: lock ownership uncertain: %w", label, err))
			return
		}
		if err := execTimedSQL(db, rollbackTimeout, query); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", label, safeDBError(err)))
		}
	}
	if dropUser {
		cleanup("cleanup user", "DROP USER IF EXISTS "+quoteUserHost(name, host))
	}
	if dropDB {
		cleanup("cleanup database", "DROP DATABASE IF EXISTS "+quoteIdent(name))
	}
	// Check both independently, even when every DROP failed or a resource was
	// excluded because CREATE reported an existing resource.
	exists, err := userExistsTimed(db, name, host, rollbackTimeout)
	if err != nil {
		errs = append(errs, fmt.Errorf("verify user cleanup: %w", safeDBError(err)))
	} else if exists {
		errs = append(errs, errors.New("user remains; manual reconciliation required"))
	}
	exists, err = databaseExistsTimed(db, name, rollbackTimeout)
	if err != nil {
		errs = append(errs, fmt.Errorf("verify database cleanup: %w", safeDBError(err)))
	} else if exists {
		errs = append(errs, errors.New("database remains; manual reconciliation required"))
	}
	return errors.Join(errs...)
}
