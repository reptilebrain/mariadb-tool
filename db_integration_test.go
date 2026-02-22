package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestProcessDatabaseMariaDBIntegration(t *testing.T) {
	if os.Getenv("MARIADB_TOOL_INTEGRATION") != "1" {
		t.Skip("set MARIADB_TOOL_INTEGRATION=1 to run integration tests")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	containerName := fmt.Sprintf("mariadb-tool-it-%d", time.Now().UnixNano())
	rootPassword := "testrootpass"
	port := "33316"

	run := exec.Command(
		"docker", "run", "-d",
		"--name", containerName,
		"-e", "MARIADB_ROOT_PASSWORD="+rootPassword,
		"-p", port+":3306",
		"mariadb:11",
	)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() {
		cmd := exec.Command("docker", "rm", "-f", containerName)
		_, _ = cmd.CombinedOutput()
	})

	cfg := map[string]string{
		"username": "root",
		"password": rootPassword,
		"hostname": "127.0.0.1",
		"port":     port,
	}

	var db *sql.DB
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		db, err = openDB(cfg, 3*time.Second)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("failed to connect MariaDB in container: %v", err)
	}
	defer db.Close()

	opts := Options{
		UserHost:  "localhost",
		Timeout:   6 * time.Second,
		Normalize: true,
	}

	res, err := processDatabase(db, opts, "example.com")
	if err != nil {
		t.Fatalf("processDatabase failed: %v", err)
	}
	if res.Status != StatusCreated {
		t.Fatalf("unexpected status: got %v want %v", res.Status, StatusCreated)
	}
	if res.Name != "example_com" {
		t.Fatalf("unexpected normalized name: got %q", res.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
