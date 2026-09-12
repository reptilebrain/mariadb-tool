package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExistingSensitiveFilesBecomePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode enforcement")
	}
	for _, name := range []string{"config.ini", "accounts.csv", "error.log"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte("existing\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0644); err != nil {
				t.Fatal(err)
			}
			var err error
			switch name {
			case "config.ini":
				err = writePrivateFile(path, []byte("[mariadb]\nusername=root\npassword=test\n"))
			case "accounts.csv":
				err = saveToCSV(path, "example", "example", "test")
			case "error.log":
				logError(path, "test error")
			}
			if err != nil {
				t.Fatal(err)
			}
			st, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if st.Mode().Perm() != 0600 {
				t.Fatalf("mode %o", st.Mode().Perm())
			}
		})
	}
}
func TestConfigRejectsInsecureMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	path := filepath.Join(t.TempDir(), "config.ini")
	os.WriteFile(path, []byte("[mariadb]\nusername=root\npassword=secret\n"), 0600)
	os.Chmod(path, 0644)
	if _, err := loadConfig(path, "mariadb"); err == nil {
		t.Fatal("insecure config accepted")
	}
	os.Chmod(path, 0600)
	if _, err := loadConfig(path, "mariadb"); err != nil {
		t.Fatal(err)
	}
}
func TestPrivateFileRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	os.WriteFile(target, []byte("original"), 0644)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(link, []byte("replacement")); err == nil {
		t.Fatal("symlink accepted")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "original" {
		t.Fatal("symlink target changed")
	}
}
func TestDefaultPathsFailClosed(t *testing.T) {
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "HOME", "USERPROFILE"} {
		t.Setenv(key, "")
	}
	if _, err := defaultPaths(); err == nil {
		t.Fatal("missing home must fail")
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	if _, err := defaultPaths(); err == nil {
		t.Fatal("relative XDG must fail")
	}
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("XDG_DATA_HOME", home)
	t.Setenv("XDG_STATE_HOME", home)
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.ConfigPath, paths.ErrorLog, paths.CSVPath} {
		if !filepath.IsAbs(path) || !strings.HasPrefix(path, home) {
			t.Fatal("unsafe default")
		}
	}
}
