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
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func configFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadConfig(filename, section string) (map[string]string, error) {
	cfg := make(map[string]string)

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	wantSection := strings.ToLower(strings.TrimSpace(section))
	curSection := ""

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			curSection = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if curSection != wantSection {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		cfg[k] = v
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func initializeConfig(path string) error {
	if configFileExists(path) {
		fmt.Print("config.ini already exists. Overwrite? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			return nil
		}
	}

	user := promptDefault("Enter MariaDB username", "root")
	pass, err := promptSecret("Enter MariaDB password")
	if err != nil {
		return err
	}
	host := promptDefault("Enter MariaDB hostname", "localhost")
	port := promptDefault("Enter MariaDB port", "3306")

	content := fmt.Sprintf(
		"[mariadb]\nusername=%s\npassword=%s\nhostname=%s\nport=%s\n",
		user, pass, host, port,
	)

	return os.WriteFile(path, []byte(content), 0600)
}

func promptDefault(label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	var v string
	fmt.Scanln(&v)
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	return v
}

func promptSecret(label string) (string, error) {
	fmt.Printf("%s: ", label)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		var v string
		fmt.Scanln(&v)
		return strings.TrimSpace(v), nil
	}
	b, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
