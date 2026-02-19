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

	_ "github.com/go-sql-driver/mysql"
)

var defaultTimeout = 6 * time.Second

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

func openDB(cfg map[string]string, timeout time.Duration) (*sql.DB, error) {
	user := cfg["username"]
	pass := cfg["password"]
	host := cfg["hostname"]
	port := cfg["port"]

	if user == "" || host == "" || port == "" {
		return nil, fmt.Errorf("config missing required fields (username/hostname/port)")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=true&loc=Local",
		user, pass, host, port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
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
	grantee := fmt.Sprintf("'%s'@'%s'", user, host)

	var one int
	err := db.QueryRowContext(ctx,
		`SELECT 1
		 FROM information_schema.USER_PRIVILEGES
		 WHERE GRANTEE = ?
		 LIMIT 1`, grantee,
	).Scan(&one)

	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func dbOrUserExists(ctx context.Context, db *sql.DB, name, host string) (bool, bool, error) {

	// Check database existence
	var tmp string
	dbErr := db.QueryRowContext(ctx,
		"SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?",
		name,
	).Scan(&tmp)

	var dbExists bool
	switch {
	case dbErr == nil:
		dbExists = true
	case errors.Is(dbErr, sql.ErrNoRows):
		dbExists = false
	default:
		return false, false, fmt.Errorf("check db exists: %w", dbErr)
	}

	// Check user existence via information_schema
	uExists, err := userExists(ctx, db, name, host)
	if err != nil {
		return false, false, fmt.Errorf("check user exists: %w", err)
	}

	return dbExists, uExists, nil
}

/* ===============================
   Main creation logic
================================= */

func processDatabase(db *sql.DB, opts Options, inputName string) (*CreateResult, error) {

	if opts.UserHost == "" {
		opts.UserHost = "localhost"
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

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	dbExists, userExists, err := dbOrUserExists(ctx, db, name, opts.UserHost)
	if err != nil {
		return nil, err
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

	// CREATE DATABASE
	if err := execSQL(ctx, db, "CREATE DATABASE "+quoteIdent(name)); err != nil {
		return nil, fmt.Errorf("create database %s: %w", name, err)
	}

	// CREATE USER
	createUserSQL := "CREATE USER " + quoteUserHost(name, opts.UserHost) +
		" IDENTIFIED BY '" + escapeSQLStringLiteral(pw) + "'"

	if err := execSQL(ctx, db, createUserSQL); err != nil {
		_ = execSQL(ctx, db, "DROP DATABASE "+quoteIdent(name))
		return nil, fmt.Errorf("create user %s: %w", quoteUserHost(name, opts.UserHost), err)
	}

	// GRANT
	grantSQL := "GRANT ALL PRIVILEGES ON " + quoteIdent(name) +
		".* TO " + quoteUserHost(name, opts.UserHost)

	if err := execSQL(ctx, db, grantSQL); err != nil {
		_ = execSQL(ctx, db, "DROP USER "+quoteUserHost(name, opts.UserHost))
		_ = execSQL(ctx, db, "DROP DATABASE "+quoteIdent(name))
		return nil, fmt.Errorf("grant privileges for %s: %w", name, err)
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
			fmt.Printf("⚠️  %s\n", res.Message)
		case StatusDryRun:
			fmt.Printf("✅ DRY-RUN OK: %s\n", res.Name)
		case StatusCreated:
			fmt.Printf("✅ Success: %s created.\n", res.Name)
			fmt.Printf("   Username: %s\n   Host:     %s\n   Password: %s\n",
				res.Username, res.UserHost, res.Password)
		}
	}

	return sc.Err()
}

func execSQL(ctx context.Context, db *sql.DB, query string) error {
	_, err := db.ExecContext(ctx, query)
	return err
}
