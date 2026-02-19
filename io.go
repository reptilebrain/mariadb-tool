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
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	configFilename = "config.ini"
	errorLogName   = "error.log"
	csvName        = "accounts.csv"
)

func saveToCSV(path, dbName, userName, password string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600)
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
		dbName, userName, password,
	}); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func logError(path string, msg string) {
	_ = os.MkdirAll(filepath.Dir(path), 0700)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	logLine := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	_, _ = f.WriteString(logLine)
}
