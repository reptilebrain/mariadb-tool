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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appName = "mariadb-tool"

type DefaultPaths struct {
	ConfigPath string
	ErrorLog   string
	CSVPath    string
}

func xdgDir(envVar string, fallbackParts ...string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	parts := append([]string{home}, fallbackParts...)
	return filepath.Join(parts...), nil
}

func defaultPaths() (DefaultPaths, error) {
	cfgHome, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return DefaultPaths{}, err
	}
	dataHome, err := xdgDir("XDG_DATA_HOME", ".local", "share")
	if err != nil {
		return DefaultPaths{}, err
	}
	stateHome, err := xdgDir("XDG_STATE_HOME", ".local", "state")
	if err != nil {
		return DefaultPaths{}, err
	}

	return DefaultPaths{
		ConfigPath: filepath.Join(cfgHome, appName, "config.ini"),
		CSVPath:    filepath.Join(dataHome, appName, "accounts.csv"),
		ErrorLog:   filepath.Join(stateHome, appName, "error.log"),
	}, nil
}

func ensureParentDir(path string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, mode)
}

func configFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadConfig(filename, section string) (map[string]string, error) {
	config := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	wantSection := "[" + section + "]"
	inSection := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.EqualFold(line, wantSection)
			continue
		}
		if !inSection {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k != "" {
			config[k] = v
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(config) == 0 {
		return nil, fmt.Errorf("missing or empty section [%s] in %s", section, filename)
	}
	return config, nil
}

func initializeConfig(path string) error {
	if err := ensureParentDir(path, 0700); err != nil {
		return err
	}

	// If exists: ask before overwrite (simple CLI confirm)
	if configFileExists(path) {
		fmt.Printf("%s already exists. Overwrite? (y/N): ", path)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			return nil
		}
	}

	var user, pass, host, port string
	fmt.Print("Enter MariaDB root username [root]: ")
	fmt.Scanln(&user)
	if strings.TrimSpace(user) == "" {
		user = "root"
	}

	fmt.Print("Enter MariaDB root password: ")
	fmt.Scanln(&pass)

	fmt.Print("Enter MariaDB hostname [localhost]: ")
	fmt.Scanln(&host)
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}

	fmt.Print("Enter MariaDB port [3306]: ")
	fmt.Scanln(&port)
	if strings.TrimSpace(port) == "" {
		port = "3306"
	}

	content := fmt.Sprintf(
		"[mariadb]\nusername=%s\npassword=%s\nhostname=%s\nport=%s\n",
		user, pass, host, port,
	)

	// 0600 because it contains creds
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}

	fmt.Printf("✅ %s created (0600).\n", path)
	return nil
}

func validateNotEmptyPaths(p DefaultPaths) error {
	if p.ConfigPath == "" || p.ErrorLog == "" || p.CSVPath == "" {
		return errors.New("internal error: empty default paths")
	}
	return nil
}
