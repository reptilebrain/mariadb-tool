package main

import (
	"bytes"
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Only walk this test's private root. Never inspect the developer's home.
func snapshotTestFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		snapshot[relative] = info.Mode().String()
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snapshot[relative] += string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunLeavesDatabaseAndFilesUntouched(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent outputs", true: "existing outputs"}[existing], func(t *testing.T) {
			root := t.TempDir()
			opts := Options{Timeout: time.Second, DryRun: true, Normalize: true, ExportCSV: true,
				CSVPath:      filepath.Join(root, "output with spaces", "accounts.csv"),
				ErrorLogPath: filepath.Join(root, "output with spaces", "error.log")}
			if existing {
				writeTestFile(t, opts.CSVPath, []byte("existing credential bytes\r\n"))
				writeTestFile(t, opts.ErrorLogPath, []byte("existing log bytes\n"))
			}
			before := snapshotTestFiles(t, root)
			state := &fakeState{}
			res, err := processDatabase(fakeDB(t, state), opts, "example.com")
			if err != nil || res.Status != StatusDryRun {
				t.Fatalf("dry run: result=%+v err=%v", res, err)
			}
			if len(state.calls) != 0 || state.database || state.user {
				t.Fatal("dry run executed DDL or changed database state")
			}
			if res.CSVExported {
				t.Fatal("dry run exported credentials")
			}
			if after := snapshotTestFiles(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("dry run changed filesystem contents or permissions")
			}
			output := captureStdout(t, func() { printResult(opts, res) })
			if strings.Contains(output, "Password:") || strings.Contains(output, res.Password) {
				t.Fatal("dry run printed generated credentials")
			}
		})
	}
}

func TestConfigParsingPreservesInputWithSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config folder", "connection settings.ini")
	data := []byte("# comment\r\n[other]\r\nusername=ignored\r\n[MaRiAdB]\r\n username = admin \r\npassword=a=b!#%&\r\nhostname=localhost\r\nport=3306\r\n; comment\r\nmalformed line\r\n[other]\r\npassword=ignored\r\n")
	writeTestFile(t, path, data)
	cfg, err := loadConfig(path, "mariadb")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"username": "admin", "password": "a=b!#%&", "hostname": "localhost", "port": "3306"}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("config = %#v, want %#v", cfg, want)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, after) {
		t.Fatal("loading configuration changed its bytes")
	}
}

func TestConfigReadFailures(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		missing       bool
	}{
		{"missing file", "", true}, {"missing section", "[other]\nusername=admin\n", false},
		{"empty section", "[mariadb]\n# empty\n", false},
		{"scanner limit", "[mariadb]\npassword=" + strings.Repeat("x", 70*1024), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.ini")
			if !tc.missing {
				writeTestFile(t, path, []byte(tc.content))
			}
			if _, err := loadConfig(path, "mariadb"); err == nil {
				t.Fatal("expected config error")
			}
			if !tc.missing {
				after, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(after) != tc.content {
					t.Fatal("failed load changed file contents")
				}
			}
		})
	}
}

func TestXDGAndHomePathsWithSpaces(t *testing.T) {
	for _, useXDG := range []bool{false, true} {
		t.Run(map[bool]string{false: "home fallback", true: "explicit XDG"}[useXDG], func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home with spaces")
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			dirs := map[string]string{
				"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
				"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
				"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
			}
			for key := range dirs {
				value := ""
				if useXDG {
					value = filepath.Join(root, key+" with spaces")
					dirs[key] = value
				}
				t.Setenv(key, value)
			}
			got, err := defaultPaths()
			if err != nil {
				t.Fatal(err)
			}
			want := DefaultPaths{
				ConfigPath: filepath.Join(dirs["XDG_CONFIG_HOME"], appName, "config.ini"),
				CSVPath:    filepath.Join(dirs["XDG_DATA_HOME"], appName, "accounts.csv"),
				ErrorLog:   filepath.Join(dirs["XDG_STATE_HOME"], appName, "error.log"),
			}
			if got != want {
				t.Fatalf("paths=%+v want %+v", got, want)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatal("resolving default paths created files")
			}
		})
	}
}

func TestBatchInputIntegrityAndDryRunSummary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "batch inputs", "names with spaces.txt")
	original := []byte("# ignored\r\n\r\nexample.com # trailing comment\r\n; ignored\r\nother.example\r\n")
	writeTestFile(t, path, original)
	opts := Options{Timeout: time.Second, DryRun: true, Normalize: true, ExportCSV: true,
		CSVPath: filepath.Join(root, "export output", "accounts.csv"), ErrorLogPath: filepath.Join(root, "error output", "error.log")}
	before := snapshotTestFiles(t, root)
	state := &fakeState{}
	output := captureStdout(t, func() {
		if err := processFile(fakeDB(t, state), opts, path); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Created: 0\nSkipped: 0\nFailed: 0") || !strings.Contains(output, "Dry-run: 2") {
		t.Fatalf("wrong summary: %s", output)
	}
	if len(state.calls) != 0 {
		t.Fatal("dry-run batch executed DDL")
	}
	if after := snapshotTestFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("batch input or output paths changed")
	}
}

func TestBatchInputFailures(t *testing.T) {
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "missing file", false: "scanner limit"}[missing], func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "batch input.txt")
			data := []byte(strings.Repeat("a", 70*1024))
			if !missing {
				writeTestFile(t, path, data)
			}
			state := &fakeState{}
			opts := Options{Timeout: time.Second, ErrorLogPath: filepath.Join(root, "error.log")}
			captureStdout(t, func() {
				if err := processFile(fakeDB(t, state), opts, path); err == nil {
					t.Fatal("expected batch read failure")
				}
			})
			if len(state.calls) != 0 {
				t.Fatal("bad input caused DDL")
			}
			if !missing {
				after, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, after) {
					t.Fatal("failed batch changed input")
				}
			}
		})
	}
}

func TestCSVAppendPreservesExistingBytesAndValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export with spaces", "accounts file.csv")
	prefix := []byte("Timestamp,Database,Username,Password\nold,existing,existing,existing-value\n")
	writeTestFile(t, path, prefix)
	// CSV metacharacters exercise serialization; none are real credentials.
	password := "test,quoted\"value"
	if err := saveToCSV(path, "new_db", "new_user", password); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, prefix) {
		t.Fatal("append changed existing bytes")
	}
	rows, err := csv.NewReader(bytes.NewReader(after)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || !reflect.DeepEqual(rows[2][1:], []string{"new_db", "new_user", password}) {
		t.Fatalf("CSV round trip failed: %#v", rows)
	}
}

func TestCSVOutputFailuresPreserveExistingFiles(t *testing.T) {
	for _, kind := range []string{"empty path", "directory target", "file blocks parent"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			target := ""
			switch kind {
			case "directory target":
				target = root
			case "file blocks parent":
				blocker := filepath.Join(root, "parent with spaces")
				writeTestFile(t, blocker, []byte("do not overwrite this file"))
				target = filepath.Join(blocker, "accounts.csv")
			}
			before := snapshotTestFiles(t, root)
			if err := saveToCSV(target, "example", "example", "test-only"); err == nil {
				t.Fatal("expected output error")
			}
			if after := snapshotTestFiles(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("failed export changed existing files")
			}
		})
	}
}
