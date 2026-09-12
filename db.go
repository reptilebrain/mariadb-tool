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
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var defaultTimeout = 6 * time.Second
var rollbackTimeout = 3 * time.Second

type Options struct {
	CreateName        string
	FileList          string
	Init              bool
	ConfigPath        string
	UserHost          string
	AllowWildcardHost bool
	Timeout           time.Duration
	ExportCSV         bool
	CSVPath           string
	ErrorLogPath      string
	DryRun            bool
	TLS               string
	TLSCA             string
	Socket            string
	Normalize         bool
}

type CreateStatus int

const (
	StatusUnknown CreateStatus = iota
	StatusSkipped
	StatusDryRun
	StatusCreated
)

type CreateResult struct {
	Status        CreateStatus
	RequestedName string
	Name          string
	Username      string
	UserHost      string
	Password      string
	Message       string
	CSVExported   bool
}

func validateOptions(opts Options) error {
	if opts.Timeout <= 0 {
		return fmt.Errorf("invalid timeout %s: must be > 0", opts.Timeout)
	}
	return nil
}

func openDB(cfg map[string]string, timeout time.Duration) (*sql.DB, error) {
	driverCfg, err := connectionConfig(cfg, timeout)
	if err != nil {
		return nil, err
	}
	connector, err := mysql.NewConnector(driverCfg)
	if err != nil {
		return nil, safeDBError(err)
	}
	db := sql.OpenDB(connector)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, safeDBError(err)
	}

	return db, nil
}

/* ===============================
   Host validation
================================= */

var hostNoWildcardRe = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
var hostWildcardRe = regexp.MustCompile(`^[a-zA-Z0-9.%_-]+$`)

func validateUserHost(user, host string, allowWildcards bool) error {
	if err := validateIdentifier(user); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return errors.New("empty host")
	}
	if len(host) > 255 {
		return errors.New("host too long")
	}

	if !allowWildcards {
		if strings.Contains(host, "%") || strings.Contains(host, "_") {
			return fmt.Errorf("wildcard host not allowed ('%%' or '_' found). Use -allow-wildcard-host to permit it")
		}
		if !hostNoWildcardRe.MatchString(host) {
			return fmt.Errorf("invalid host '%s' (allowed: a-z A-Z 0-9 . -)", host)
		}
		return nil
	}

	if !hostWildcardRe.MatchString(host) {
		return fmt.Errorf("invalid host '%s' (allowed: a-z A-Z 0-9 . %% _ -)", host)
	}

	return nil
}

func quoteUserHost(user, host string) string {
	return fmt.Sprintf("'%s'@'%s'", user, host)
}

func escapeSQLStringLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

/* ===============================
   Existence checks
================================= */

func userExists(ctx context.Context, db *sql.DB, user, host string) (bool, error) {

	var one int
	err := db.QueryRowContext(ctx,
		"SELECT 1 FROM mysql.user WHERE User = ? AND Host = ? LIMIT 1", user, host,
	).Scan(&one)

	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

/* ===============================
   Main creation logic
================================= */

func processDatabase(db *sql.DB, opts Options, inputName string) (*CreateResult, error) {

	opts.UserHost = strings.TrimSpace(opts.UserHost)
	if opts.UserHost == "" {
		opts.UserHost = "localhost"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}

	requested := strings.TrimSpace(inputName)
	if requested == "" {
		return nil, errors.New("empty name")
	}

	name := requested

	if opts.Normalize {
		if err := validateRawNameForNormalization(requested); err != nil {
			return nil, err
		}
		name = normalizeName(requested)
		if name == "" {
			return nil, fmt.Errorf("name '%s' normalizes to empty identifier", requested)
		}
	}

	if err := validateUserHost(name, opts.UserHost, opts.AllowWildcardHost); err != nil {
		return nil, err
	}

	res := &CreateResult{
		Status:        StatusUnknown,
		RequestedName: requested,
		Name:          name,
		Username:      name,
		UserHost:      opts.UserHost,
	}

	lock, err := acquireNameLock(db, name, opts.Timeout)
	if err != nil {
		return nil, err
	}
	defer lock.close()
	dbExists, err := databaseExistsTimed(db, name, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("check database: %w", safeDBError(err))
	}
	userExists, err := userExistsTimed(db, name, opts.UserHost, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("check user: %w", safeDBError(err))
	}

	if dbExists || userExists {
		res.Status = StatusSkipped
		switch {
		case dbExists && userExists:
			res.Message = fmt.Sprintf("Skipping '%s': database exists and user %s exists",
				name, quoteUserHost(name, opts.UserHost))
		case dbExists:
			res.Message = fmt.Sprintf("Skipping '%s': database exists (will not create user)", name)
		case userExists:
			res.Message = fmt.Sprintf("Skipping '%s': user %s exists (will not create database)",
				name, quoteUserHost(name, opts.UserHost))
		}
		return res, nil
	}

	pw, err := generatePassword(20)
	if err != nil {
		return nil, err
	}
	res.Password = pw

	if opts.DryRun {
		res.Status = StatusDryRun
		return res, nil
	}

	// The absence snapshot is taken under a separate advisory-lock connection.
	// Mark attempts before executing: a lost response does not prove failure.
	attemptedDB, attemptedUser := false, false
	fail := func(stage string, cause error) (*CreateResult, error) {
		return nil, errors.Join(fmt.Errorf("%s: %w", stage, safeDBError(cause)),
			reconcile(db, lock, name, opts.UserHost, attemptedDB, attemptedUser))
	}
	if err := lock.check(); err != nil {
		return fail("creation lock lost", err)
	}
	attemptedDB = true
	if err := execTimedSQL(db, opts.Timeout, "CREATE DATABASE "+quoteIdent(name)); err != nil {
		if mysqlErrorNumber(err) == 1007 {
			attemptedDB = false
		}
		return fail("create database", err)
	}
	if err := lock.check(); err != nil {
		return fail("creation lock lost", err)
	}
	attemptedUser = true
	createUserSQL := "CREATE USER " + quoteUserHost(name, opts.UserHost) +
		" IDENTIFIED BY '" + escapeSQLStringLiteral(pw) + "'"
	if err := execTimedSQL(db, opts.Timeout, createUserSQL); err != nil {
		// 1396 can indicate an externally created account. Never delete it.
		if mysqlErrorNumber(err) == 1396 {
			attemptedUser = false
		}
		return fail("create user", err)
	}
	if err := lock.check(); err != nil {
		return fail("creation lock lost", err)
	}
	grantSQL := "GRANT ALL PRIVILEGES ON " + quoteIdent(strings.ReplaceAll(name, "_", "\\_")) +
		".* TO " + quoteUserHost(name, opts.UserHost)
	if err := execTimedSQL(db, opts.Timeout, grantSQL); err != nil {
		return fail("grant privileges", err)
	}

	res.Status = StatusCreated

	if opts.ExportCSV {
		if err := saveToCSV(opts.CSVPath, name, name, pw); err != nil {
			msg := fmt.Sprintf("WARNING: failed to export CSV for %s: %v", name, err)
			logError(opts.ErrorLogPath, msg)
		} else {
			res.CSVExported = true
		}
	}

	return res, nil
}

/* ===============================
   Batch mode
================================= */

func processFile(db *sql.DB, opts Options, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineNo := 0
	created, skipped, failed, dryRun := 0, 0, 0, 0

	for sc.Scan() {
		lineNo++
		raw := strings.TrimSpace(sc.Text())

		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ";") {
			continue
		}

		if i := strings.IndexAny(raw, "#;"); i >= 0 {
			raw = strings.TrimSpace(raw[:i])
			if raw == "" {
				continue
			}
		}

		res, err := processDatabase(db, opts, raw)
		if err != nil {
			failed++
			msg := fmt.Sprintf("Line %d (%s): %v", lineNo, raw, err)
			fmt.Println("❌", msg)
			logError(opts.ErrorLogPath, msg)
			continue
		}

		if opts.Normalize && res.RequestedName != "" && res.RequestedName != res.Name {
			fmt.Printf("   Requested: %s -> Normalized: %s\n",
				res.RequestedName, res.Name)
		}

		switch res.Status {
		case StatusSkipped:
			skipped++
			fmt.Printf("⚠️  %s\n", res.Message)
		case StatusDryRun:
			dryRun++
			fmt.Printf("✅ DRY-RUN OK: %s\n", res.Name)
		case StatusCreated:
			created++
			fmt.Printf("✅ Success: %s created.\n", res.Name)
			fmt.Printf("   Username: %s\n   Host:     %s\n   Password: %s\n",
				res.Username, res.UserHost, res.Password)
		}
	}

	scanErr := sc.Err()
	if scanErr != nil {
		logError(opts.ErrorLogPath, fmt.Sprintf("Batch read failed: %v", scanErr))
		failed++
	}
	fmt.Printf("Batch complete:\nCreated: %d\nSkipped: %d\nFailed: %d\n", created, skipped, failed)
	if opts.DryRun {
		fmt.Printf("Dry-run: %d\n", dryRun)
	}
	if failed > 0 {
		return errors.Join(fmt.Errorf("batch had %d failure(s)", failed), scanErr)
	}
	return nil
}

func execSQL(ctx context.Context, db *sql.DB, query string) error {
	_, err := db.ExecContext(ctx, query)
	return err
}
