// mariadb-tool
// Copyright (C) 2026 P-A Jonasson
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY.
//
// See the LICENSE file in the project root for details.

package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"
)

func logError(path, msg string) {
	if path == "" {
		return
	}
	if err := ensureParentDir(path, 0700); err != nil {
		fmt.Fprintln(os.Stderr, "Cannot create private output directory:", err)
		return
	}

	// 0600: log may contain sensitive operational info
	f, err := openPrivateFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot open private error log:", err)
		return
	}
	defer f.Close()

	logLine := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	if _, err := f.WriteString(logLine); err != nil {
		fmt.Fprintln(os.Stderr, "Cannot write error log:", err)
	}
}

func saveToCSV(path, dbName, userName, password string) error {
	if path == "" {
		return fmt.Errorf("csv path is empty")
	}

	if err := ensureParentDir(path, 0700); err != nil {
		return err
	}

	// 0600: CSV contains credentials
	f, err := openPrivateFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	if info.Size() == 0 {
		if err := w.Write([]string{"Timestamp", "Database", "Username", "Password"}); err != nil {
			return err
		}
	}

	if err := w.Write([]string{
		time.Now().Format("2006-01-02 15:04"),
		dbName,
		userName,
		password,
	}); err != nil {
		return err
	}

	w.Flush()
	return w.Error()
}

// Parent directories must be trusted: reject symlinks and tighten existing files
// before writing. Do not truncate a credential file until chmod succeeds.
func openPrivateFile(path string, flags int) (*os.File, error) {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("private file must be regular: %s", path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(path, flags&^os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) { f.Close(); return nil, err }
	opened, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if !opened.Mode().IsRegular() || (info != nil && !os.SameFile(info, opened)) {
		return fail(fmt.Errorf("private file changed while opening: %s", path))
	}
	if err := f.Chmod(0600); err != nil {
		return fail(err)
	}
	if flags&os.O_TRUNC != 0 {
		if err := f.Truncate(0); err != nil {
			return fail(err)
		}
	}
	return f, nil
}
func writePrivateFile(path string, data []byte) error {
	f, err := openPrivateFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	return errors.Join(writeErr, f.Close())
}
func checkConfigPermissions(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("config must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return errors.New("config permissions are insecure; run chmod 600 on the config file")
	}
	return nil
}
