/*
 * Copyright (C) 2026 P-A Jonasson
 * * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 * * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 */

package main

import (
	"bufio"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	User, Pass, Host, Port string
}

func loadConfig(filename string) Config {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		content := "[database]\nuser=root\npass=\nhost=127.0.0.1\nport=3306\n"
		os.WriteFile(filename, []byte(content), 0644)
		fmt.Println("Created config.ini. Fill in your credentials and run again.")
		os.Exit(0)
	}
	conf := Config{User: "root", Host: "127.0.0.1", Port: "3306"}
	file, _ := os.Open(filename)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "user":
				conf.User = val
			case "pass":
				conf.Pass = val
			case "host":
				conf.Host = val
			case "port":
				conf.Port = val
			}
		}
	}
	return conf
}

func generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#%&"
	password := make([]byte, length)
	for i := range password {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		password[i] = charset[num.Int64()]
	}
	return string(password)
}

func logError(msg string) {
	f, _ := os.OpenFile("error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " - " + msg + "\n")
}

func saveToCSV(dbName, userName, password string) {
	fileName := "accounts.csv"

	// Öppna filen (skapa om den inte finns)
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		logError("Could not open CSV: " + err.Error())
		return
	}
	defer f.Close()

	// Kolla om filen är tom
	info, _ := f.Stat()
	isNewOrEmpty := info.Size() == 0

	w := csv.NewWriter(f)
	defer w.Flush()

	// Skriv rubriker om filen är ny eller rensad
	if isNewOrEmpty {
		w.Write([]string{"Timestamp", "Database", "Username", "Password"})
	}

	w.Write([]string{time.Now().Format("2006-01-02 15:04"), dbName, userName, password})
}

func createDBAndUser(conf Config, dbName string) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", conf.User, conf.Pass, conf.Host, conf.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logError(fmt.Sprintf("Connection error for %s: %v", dbName, err))
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logError(fmt.Sprintf("Ping failed for %s: %v", dbName, err))
		return
	}

	// Double check: Does User OR Database already exist?
	var userExists, dbExists int
	db.QueryRow("SELECT COUNT(*) FROM mysql.user WHERE user = ?", dbName).Scan(&userExists)
	db.QueryRow("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?", dbName).Scan(&dbExists)

	if userExists > 0 || dbExists > 0 {
		msg := fmt.Sprintf("SKIP: User or Database '%s' already exists.", dbName)
		fmt.Println(msg)
		logError(msg)
		return
	}

	pass := generatePassword(16)
	// Use backticks for database names to support dots/special characters
	queries := []string{
		fmt.Sprintf("CREATE DATABASE `%s`", dbName),
		fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s'", dbName, pass),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'", dbName, dbName),
		"FLUSH PRIVILEGES",
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			logError(fmt.Sprintf("[%s] SQL Error during '%s': %v", dbName, q, err))
			return
		}
	}

	saveToCSV(dbName, dbName, pass)
	fmt.Printf("Created: %s\n", dbName)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  -c <name>      Create single database/user")
		fmt.Println("  -f <file.txt>  Batch processing from file")
		return
	}

	conf := loadConfig("config.ini")
	flag, val := os.Args[1], os.Args[2]

	switch flag {
	case "-c":
		createDBAndUser(conf, val)
	case "-f":
		file, err := os.Open(val)
		if err != nil {
			logError("Could not open batch file: " + val)
			return
		}
		defer file.Close()
		s := bufio.NewScanner(file)
		for s.Scan() {
			name := strings.TrimSpace(s.Text())
			if name != "" {
				createDBAndUser(conf, name)
			}
		}
	default:
		fmt.Println("Unknown flag. Use -c or -f.")
	}
}
