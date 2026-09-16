package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"github.com/go-sql-driver/mysql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeState struct {
	contexts                 []context.Context
	acquireErr               error
	lockKeys                 []string
	mu                       sync.Mutex
	database, user           bool
	failure                  string
	cause                    error
	applyFailure             bool
	dropUserFail, dropDBFail bool
	verifyFail               bool
	lockLost                 bool
	calls                    []string
	deadlines                []time.Duration
	delay                    time.Duration
}
type fakeConnector struct{ state *fakeState }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) { return &fakeConn{c.state}, nil }
func (c fakeConnector) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type fakeConn struct{ state *fakeState }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }

type fakeRows struct{ values []driver.Value }

func (r *fakeRows) Columns() []string { return []string{"v"} }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	dest[0] = r.values[0]
	r.values = r.values[1:]
	return nil
}
func (c *fakeConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var value driver.Value
	switch {
	case strings.Contains(q, "GET_LOCK"):
		s.lockKeys = append(s.lockKeys, args[0].Value.(string))
		if s.acquireErr != nil {
			return nil, s.acquireErr
		}
		value = int64(1)
	case strings.Contains(q, "RELEASE_LOCK"):
		value = int64(1)
	case strings.Contains(q, "IS_USED_LOCK"):
		value = !(s.lockLost && len(s.calls) > 0)
	default:
		if s.verifyFail && len(s.calls) > 0 {
			return nil, io.EOF
		}
		if d, ok := ctx.Deadline(); ok {
			s.deadlines = append(s.deadlines, time.Until(d))
			s.contexts = append(s.contexts, ctx)
		} else {
			return nil, errors.New("missing deadline")
		}
		if strings.Contains(q, "SCHEMATA") {
			if s.database {
				value = "example"
			}
		} else if strings.Contains(q, "mysql.user") {
			if s.user {
				value = int64(1)
			}
		} else {
			return nil, errors.New("unexpected query")
		}
	}
	if value == nil {
		return &fakeRows{}, nil
	}
	return &fakeRows{[]driver.Value{value}}, nil
}
func (c *fakeConn) ExecContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, q)
	d, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("missing deadline")
	}
	s.deadlines = append(s.deadlines, time.Until(d))
	s.contexts = append(s.contexts, ctx)
	failed := s.failure != "" && strings.HasPrefix(q, s.failure)
	if !failed || s.applyFailure {
		switch {
		case strings.HasPrefix(q, "CREATE DATABASE"):
			s.database = true
		case strings.HasPrefix(q, "CREATE USER"):
			s.user = true
		case strings.HasPrefix(q, "DROP USER"):
			if !s.dropUserFail {
				s.user = false
			}
		case strings.HasPrefix(q, "DROP DATABASE"):
			if !s.dropDBFail {
				s.database = false
			}
		}
	}
	if strings.HasPrefix(q, "DROP USER") && s.dropUserFail {
		return nil, io.EOF
	}
	if strings.HasPrefix(q, "DROP DATABASE") && s.dropDBFail {
		return nil, io.EOF
	}
	if failed {
		return nil, s.cause
	}
	if s.delay > 0 && strings.HasPrefix(q, "CREATE") {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.delay):
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}
func fakeDB(t *testing.T, s *fakeState) *sql.DB {
	t.Helper()
	db := sql.OpenDB(fakeConnector{s})
	t.Cleanup(func() { db.Close() })
	return db
}
func TestReconciliationFailures(t *testing.T) {
	for _, stage := range []string{"CREATE DATABASE", "CREATE USER", "GRANT"} {
		for _, cause := range []error{errors.New("server rejected SQL with secret"), io.EOF, context.DeadlineExceeded} {
			for _, applied := range []bool{false, true} {
				t.Run(stage+"/"+cause.Error()+"/"+map[bool]string{true: "applied", false: "not-applied"}[applied], func(t *testing.T) {
					s := &fakeState{failure: stage, cause: cause, applyFailure: applied}
					_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
					if err == nil {
						t.Fatal("expected failure")
					}
					if strings.Contains(err.Error(), "secret") {
						t.Fatal("secret leaked")
					}
					if s.database || s.user {
						t.Fatalf("partial state remains: %v", err)
					}
					drops := strings.Join(s.calls, "\n")
					if !strings.Contains(drops, "DROP DATABASE") {
						t.Fatal("database cleanup not attempted")
					}
					if stage != "CREATE DATABASE" && !strings.Contains(drops, "DROP USER") {
						t.Fatal("user cleanup not attempted")
					}
				})
			}
		}
	}
}
func TestReconciliationAllFailuresReported(t *testing.T) {
	s := &fakeState{failure: "GRANT", cause: io.EOF, dropUserFail: true, dropDBFail: true}
	_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
	for _, want := range []string{"cleanup user", "cleanup database", "user remains", "database remains"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q: %v", want, err)
		}
	}
}
func TestReconciliationVerificationFailures(t *testing.T) {
	s := &fakeState{failure: "GRANT", cause: io.EOF, verifyFail: true}
	_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
	for _, want := range []string{"verify user cleanup", "verify database cleanup"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %s: %v", want, err)
		}
	}
}
func TestPreexistingResourcesUntouched(t *testing.T) {
	for _, s := range []*fakeState{{database: true}, {user: true}, {database: true, user: true}} {
		res, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
		if err != nil || res.Status != StatusSkipped || len(s.calls) != 0 {
			t.Fatalf("modified preexisting state: %v", err)
		}
	}
}
func TestAlreadyExistsRacePreservesResource(t *testing.T) {
	for _, tc := range []struct {
		stage  string
		number uint16
	}{{"CREATE DATABASE", 1007}, {"CREATE USER", 1396}} {
		s := &fakeState{failure: tc.stage, cause: &mysql.MySQLError{Number: tc.number}, applyFailure: true}
		_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
		if err == nil {
			t.Fatal("expected conflict")
		}
		if tc.number == 1007 && !s.database {
			t.Fatal("removed conflicting database")
		}
		if tc.number == 1396 && !s.user {
			t.Fatal("removed conflicting user")
		}
	}
}
func TestIndependentOperationTimeouts(t *testing.T) {
	s := &fakeState{}
	res, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
	if err != nil || res.Status != StatusCreated {
		t.Fatalf("creation failed: %v", err)
	}
	// Observe each query/DDL context directly rather than relying on short
	// wall-clock thresholds that can flake on loaded Windows/macOS runners.
	if len(s.contexts) != 5 {
		t.Fatalf("got %d operation contexts, want 5", len(s.contexts))
	}
	seen := map[context.Context]bool{}
	for _, ctx := range s.contexts {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("operation has no deadline")
		}
		if seen[ctx] {
			t.Fatal("operations share a timeout context")
		}
		seen[ctx] = true
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("operation context was not canceled after use")
		}
	}
}
func TestCleanupAfterExpiredOperation(t *testing.T) {
	s := &fakeState{delay: 30 * time.Millisecond}
	_, err := processDatabase(fakeDB(t, s), Options{Timeout: 10 * time.Millisecond}, "example")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wrong error: %v", err)
	}
	if s.database || s.user {
		t.Fatal("cleanup inherited canceled context")
	}
}
func TestLostLockWithholdsDestructiveCleanup(t *testing.T) {
	s := &fakeState{failure: "CREATE DATABASE", cause: io.EOF, applyFailure: true, lockLost: true}
	_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
	if err == nil || !strings.Contains(err.Error(), "withheld") || !s.database {
		t.Fatalf("unsafe lock handling: %v", err)
	}
}
func TestBatchPartialFailure(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "list.txt")
	os.WriteFile(input, []byte("bad name\nexample\nexample\n"), 0600)
	s := &fakeState{}
	output := captureStdout(t, func() {
		err := processFile(fakeDB(t, s), Options{Timeout: time.Second, ErrorLogPath: filepath.Join(dir, "error.log")}, input)
		if err == nil {
			t.Fatal("batch must return error")
		}
	})
	if !strings.Contains(output, "Created: 1\nSkipped: 1\nFailed: 1") {
		t.Fatalf("bad summary: %s", output)
	}
	log, err := os.ReadFile(filepath.Join(dir, "error.log"))
	if err != nil || !strings.Contains(string(log), "Line 1") {
		t.Fatal("missing error log")
	}
}
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = f
	t.Cleanup(func() { os.Stdout = old; f.Close() })
	fn()
	os.Stdout = old
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestGrantEscapesDatabaseWildcards(t *testing.T) {
	s := &fakeState{}
	_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example_db")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.calls[2], "example\\_db") {
		t.Fatal("GRANT underscores must be literal")
	}
}

func TestUncertainLockAcquisitionDiscardsConnection(t *testing.T) {
	s := &fakeState{acquireErr: io.EOF}
	db := fakeDB(t, s)
	if _, err := processDatabase(db, Options{Timeout: time.Second}, "example"); err == nil {
		t.Fatal("expected lock error")
	}
	if len(s.calls) != 0 {
		t.Fatal("DDL executed without lock")
	}
	if db.Stats().Idle != 0 {
		t.Fatal("uncertain lock connection returned to pool")
	}
}
func TestCaseVariantsUseSameLock(t *testing.T) {
	s := &fakeState{}
	db := fakeDB(t, s)
	for _, name := range []string{"Example", "example"} {
		if _, err := processDatabase(db, Options{Timeout: time.Second}, name); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.lockKeys) != 2 || s.lockKeys[0] != s.lockKeys[1] {
		t.Fatal("case variants bypass name lock")
	}
}
func TestFailedUserDropStillRemovesDatabase(t *testing.T) {
	s := &fakeState{failure: "GRANT", cause: io.EOF, dropUserFail: true}
	_, err := processDatabase(fakeDB(t, s), Options{Timeout: time.Second}, "example")
	if err == nil || s.database || !s.user {
		t.Fatalf("incorrect cleanup state: %v", err)
	}
}
