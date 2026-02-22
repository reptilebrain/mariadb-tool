package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestProcessDatabaseLocalMariaDBIntegration(t *testing.T) {
	if os.Getenv("MARIADB_TOOL_LOCAL_INTEGRATION") != "1" {
		t.Skip("set MARIADB_TOOL_LOCAL_INTEGRATION=1 to run local MariaDB integration test")
	}

	dsn := os.Getenv("MARIADB_TOOL_LOCAL_DSN")
	if dsn == "" {
		t.Skip("set MARIADB_TOOL_LOCAL_DSN to a local MariaDB DSN (example: root:pass@tcp(127.0.0.1:3306)/)")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql open failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping local MariaDB failed: %v", err)
	}

	opts := Options{
		UserHost:  "localhost",
		Timeout:   6 * time.Second,
		Normalize: true,
	}

	inputName := "local-it-" + time.Now().Format("20060102150405")
	res, err := processDatabase(db, opts, inputName+".example.com")
	if err != nil {
		t.Fatalf("processDatabase failed: %v", err)
	}
	if res.Status != StatusCreated {
		t.Fatalf("unexpected status: got %v want %v", res.Status, StatusCreated)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = execSQL(cleanupCtx, db, "DROP USER "+quoteUserHost(res.Name, opts.UserHost))
		_ = execSQL(cleanupCtx, db, "DROP DATABASE "+quoteIdent(res.Name))
	})

	var schemaCount int
	if err := db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?",
		res.Name,
	).Scan(&schemaCount); err != nil {
		t.Fatalf("check schema existence failed: %v", err)
	}
	if schemaCount != 1 {
		t.Fatalf("expected created schema to exist, count=%d", schemaCount)
	}

	var userCount int
	if err := db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM mysql.user WHERE User = ? AND Host = ?",
		res.Name, opts.UserHost,
	).Scan(&userCount); err != nil {
		t.Fatalf("check user existence failed: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("expected created user to exist, count=%d", userCount)
	}
}
