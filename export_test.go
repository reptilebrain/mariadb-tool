package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSingleCSVExportFailureSignalsErrorWithoutRollback(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		Timeout:      time.Second,
		ExportCSV:    true,
		CSVPath:      dir, // opening a directory as the CSV file must fail
		ErrorLogPath: filepath.Join(dir, "error.log"),
	}
	s := &fakeState{}

	res, err := processDatabase(fakeDB(t, s), opts, "example")
	if err != nil {
		t.Fatalf("creation should succeed despite export failure: %v", err)
	}
	if res.Status != StatusCreated || res.CSVExported {
		t.Fatalf("unexpected result: status=%v exported=%v", res.Status, res.CSVExported)
	}
	if credentialExportError(opts, res) == nil {
		t.Fatal("single-create path must signal a non-zero outcome for requested export failure")
	}
	if !s.database || !s.user {
		t.Fatal("successful resources must not be rolled back for an export failure")
	}

	output := captureStdout(t, func() { printResult(opts, res) })
	if !strings.Contains(output, "Password:") || !strings.Contains(output, "credential export failed") {
		t.Fatalf("credentials/export warning missing from output: %s", output)
	}
}

func TestBatchCSVExportFailureReturnsErrorAndCountsSeparately(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(input, []byte("example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := Options{
		Timeout:      time.Second,
		ExportCSV:    true,
		CSVPath:      dir, // force export failure after successful creation
		ErrorLogPath: filepath.Join(dir, "error.log"),
	}
	s := &fakeState{}

	output := captureStdout(t, func() {
		err := processFile(fakeDB(t, s), opts, input)
		if err == nil {
			t.Fatal("batch must return non-zero outcome when requested credential export fails")
		}
	})
	for _, want := range []string{"Created: 1", "Failed: 0", "Export failed: 1", "Password:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in batch output: %s", want, output)
		}
	}
	if !s.database || !s.user {
		t.Fatal("batch export failure must not roll back successfully created resources")
	}
}
