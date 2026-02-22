package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	rollbackDriverOnce  sync.Once
	rollbackDriverState *rollbackState
)

type rollbackState struct {
	rollbackCalled int32
}

type rollbackDriver struct{}

type rollbackConn struct {
	state *rollbackState
}

type emptyRows struct{}

func (d rollbackDriver) Open(_ string) (driver.Conn, error) {
	return &rollbackConn{state: rollbackDriverState}, nil
}

func (c *rollbackConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *rollbackConn) Close() error              { return nil }
func (c *rollbackConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

func (c *rollbackConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "information_schema.SCHEMATA") || strings.Contains(query, "information_schema.USER_PRIVILEGES") {
		return emptyRows{}, nil
	}
	return nil, errors.New("unexpected query: " + query)
}

func (c *rollbackConn) ExecContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	switch {
	case strings.HasPrefix(query, "CREATE DATABASE "):
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(query, "CREATE USER "):
		time.Sleep(30 * time.Millisecond)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("forced create user failure")
	case strings.HasPrefix(query, "DROP DATABASE "):
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := ctx.Deadline(); !ok {
			return nil, errors.New("rollback context missing deadline")
		}
		atomic.StoreInt32(&c.state.rollbackCalled, 1)
		return driver.RowsAffected(1), nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (emptyRows) Columns() []string           { return []string{"v"} }
func (emptyRows) Close() error                { return nil }
func (emptyRows) Next(_ []driver.Value) error { return io.EOF }

func TestProcessDatabaseRollbackUsesIndependentContext(t *testing.T) {
	rollbackDriverOnce.Do(func() {
		sql.Register("rollback_test_driver", rollbackDriver{})
	})

	state := &rollbackState{}
	rollbackDriverState = state

	db, err := sql.Open("rollback_test_driver", "")
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	defer db.Close()

	opts := Options{
		UserHost: "localhost",
		Timeout:  10 * time.Millisecond,
	}

	_, err = processDatabase(db, opts, "example_db")
	if err == nil {
		t.Fatal("expected processDatabase error")
	}
	if strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("rollback should succeed with independent context, got: %v", err)
	}
	if atomic.LoadInt32(&state.rollbackCalled) != 1 {
		t.Fatal("expected rollback DROP DATABASE to be executed")
	}
}
