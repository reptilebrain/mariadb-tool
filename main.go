package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	createFlag := flag.String("c", "", "Create single database/user")
	listFlag := flag.String("f", "", "Batch processing from file")
	initFlag := flag.Bool("i", false, "Initialize configuration")

	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  -c <name>      Create single database/user")
		fmt.Println("  -f <file.txt>  Batch processing from file")
		fmt.Println("  -i             Initialize configuration")
	}

	flag.Parse()

	if *initFlag || !configFileExists() {
		initializeConfig()
		if *initFlag {
			return
		}
	}

	config, err := loadConfig("config.ini")
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", config["username"], config["password"], config["hostname"], config["port"])
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *createFlag != "" {
		processDatabase(db, *createFlag)
	} else if *listFlag != "" {
		processFile(db, *listFlag)
	} else {
		flag.Usage()
	}
}

func loadConfig(filename string) (map[string]string, error) {
	config := make(map[string]string)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return config, scanner.Err()
}

func configFileExists() bool {
	_, err := os.Stat("config.ini")
	return !os.IsNotExist(err)
}

func initializeConfig() {
	if configFileExists() {
		fmt.Print("config.ini already exists. Overwrite? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return
		}
	}

	var user, pass, host, port string
	fmt.Print("Enter MariaDB root username [root]: ")
	fmt.Scanln(&user)
	if user == "" {
		user = "root"
	}
	fmt.Print("Enter MariaDB root password: ")
	fmt.Scanln(&pass)
	fmt.Print("Enter MariaDB hostname [localhost]: ")
	fmt.Scanln(&host)
	if host == "" {
		host = "localhost"
	}
	fmt.Print("Enter MariaDB port [3306]: ")
	fmt.Scanln(&port)
	if port == "" {
		port = "3306"
	}

	content := fmt.Sprintf("[mariadb]\nusername=%s\npassword=%s\nhostname=%s\nport=%s\n", user, pass, host, port)
	os.WriteFile("config.ini", []byte(content), 0600)
	fmt.Println("✅ config.ini created and secured.")
}

func logError(msg string) {
	f, err := os.OpenFile("error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	logLine := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	f.WriteString(logLine)
}

func processDatabase(db *sql.DB, name string) {
	var dbExists string
	dbErr := db.QueryRow("SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", name).Scan(&dbExists)
	var userExists int
	_ = db.QueryRow("SELECT COUNT(*) FROM mysql.user WHERE user = ? AND host = 'localhost'", name).Scan(&userExists)

	if dbErr == nil || userExists > 0 {
		msg := fmt.Sprintf("Skipping '%s': database or user already exists", name)
		fmt.Printf("⚠️  %s\n", msg)
		logError(msg)
		return
	}

	password := generatePassword(16)
	_, err := db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", name))
	if err != nil {
		msg := fmt.Sprintf("Error creating database %s: %v", name, err)
		fmt.Println("❌", msg)
		logError(msg)
		return
	}

	_, err = db.Exec(fmt.Sprintf("CREATE USER '%s'@'localhost' IDENTIFIED BY '%s'", name, password))
	if err != nil {
		msg := fmt.Sprintf("Error creating user %s: %v", name, err)
		fmt.Println("❌", msg)
		logError(msg)
		return
	}

	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'", name, name))
	if err != nil {
		msg := fmt.Sprintf("Error granting privileges for %s: %v", name, err)
		fmt.Println("❌", msg)
		logError(msg)
		return
	}

	fmt.Printf("✅ Success: %s created.\n", name)
	saveToCSV(name, name, password)
}

func processFile(db *sql.DB, filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			processDatabase(db, name)
		}
	}
}

func generatePassword(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#%&"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func saveToCSV(dbName, userName, password string) {
	f, err := os.OpenFile("accounts.csv", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w := csv.NewWriter(f)
	if info.Size() == 0 {
		w.Write([]string{"Timestamp", "Database", "Username", "Password"})
	}
	w.Write([]string{time.Now().Format("2006-01-02 15:04"), dbName, userName, password})
	w.Flush()
}
