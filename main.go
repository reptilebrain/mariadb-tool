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
	"flag"
	"fmt"
	"log"
	"strings"
)

func main() {
	opts := parseFlags()

	// Init or missing config => init (at XDG default unless overridden)
	if opts.Init || !configFileExists(opts.ConfigPath) {
		if err := initializeConfig(opts.ConfigPath); err != nil {
			logError(opts.ErrorLogPath, fmt.Sprintf("Config init failed: %v", err))
			log.Fatalf("Config init failed: %v", err)
		}
		if opts.Init {
			return
		}
	}

	cfg, err := loadConfig(opts.ConfigPath, "mariadb")
	if err != nil {
		logError(opts.ErrorLogPath, fmt.Sprintf("Error reading config (%s): %v", opts.ConfigPath, err))
		log.Fatalf("Error reading config: %v", err)
	}

	db, err := openDB(cfg, opts.Timeout)
	if err != nil {
		logError(opts.ErrorLogPath, fmt.Sprintf("DB connect failed: %v", err))
		log.Fatalf("DB connect failed: %v", err)
	}
	defer db.Close()

	switch {
	case opts.CreateName != "":
		name := strings.TrimSpace(opts.CreateName)
		res, err := processDatabase(db, opts, name)
		if err != nil {
			logError(opts.ErrorLogPath, fmt.Sprintf("Create failed (%s): %v", name, err))
			log.Fatalf("Failed: %v", err)
		}
		printResult(opts, res)

	case opts.FileList != "":
		if err := processFile(db, opts, opts.FileList); err != nil {
			logError(opts.ErrorLogPath, fmt.Sprintf("Batch failed (%s): %v", opts.FileList, err))
			log.Fatalf("Batch failed: %v", err)
		}

	default:
		flag.Usage()
	}
}

func parseFlags() Options {
	dp, err := defaultPaths()
	if err != nil || validateNotEmptyPaths(dp) != nil {
		// Last-resort fallback if we can't resolve XDG paths
		dp = DefaultPaths{
			ConfigPath: "config.ini",
			ErrorLog:   "error.log",
			CSVPath:    "accounts.csv",
		}
	}

	var opts Options

	flag.StringVar(&opts.CreateName, "c", "", "Create single database/user (name)")
	flag.StringVar(&opts.FileList, "f", "", "Batch processing from file (one name per line)")
	flag.BoolVar(&opts.Init, "i", false, "Initialize configuration")

	flag.StringVar(&opts.ConfigPath, "config", dp.ConfigPath, "Path to config.ini")
	flag.StringVar(&opts.UserHost, "user-host", "localhost", "Host part for created user (e.g. localhost)")
	flag.BoolVar(&opts.AllowWildcardHost, "allow-wildcard-host", false, "Allow host wildcards in -user-host (%, _)")

	flag.DurationVar(&opts.Timeout, "timeout", defaultTimeout, "Timeout per DB operation (e.g. 6s, 10s)")

	flag.BoolVar(&opts.ExportCSV, "export-csv", false, "Export created credentials to CSV (unsafe; opt-in)")
	flag.StringVar(&opts.CSVPath, "csv", dp.CSVPath, "CSV output path (used with -export-csv)")

	flag.StringVar(&opts.ErrorLogPath, "error-log", dp.ErrorLog, "Error log path")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Show what would be done, but do not execute changes")

	flag.BoolVar(&opts.Normalize, "normalize", true, "Normalize input names (e.g. hardhq.com -> hardhq_com)")

	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  -c <name>      Create single database/user")
		fmt.Println("  -f <file.txt>  Batch processing from file (one name per line)")
		fmt.Println("  -i             Initialize configuration")
		fmt.Println("")
		fmt.Println("Options:")
		flag.PrintDefaults()
	}

	flag.Parse()
	return opts
}

func printResult(opts Options, res *CreateResult) {
	if res == nil {
		return
	}

	if opts.Normalize && res.RequestedName != "" && res.RequestedName != res.Name {
		fmt.Printf("   Requested: %s\n   Normalized: %s\n", res.RequestedName, res.Name)
	}

	switch res.Status {
	case StatusSkipped:
		fmt.Printf("⚠️  %s\n", res.Message)
	case StatusDryRun:
		fmt.Printf("✅ DRY-RUN OK: %s\n", res.Name)
	case StatusCreated:
		fmt.Printf("✅ Success: %s created.\n", res.Name)
		fmt.Printf("   Username: %s\n   Host:     %s\n   Password: %s\n", res.Username, res.UserHost, res.Password)
		if opts.ExportCSV && res.CSVExported {
			fmt.Printf("   Exported:  %s\n", opts.CSVPath)
		}
	}
}
