package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func init() {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "locolm.db")

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open database: %v", err))
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS memories (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(fmt.Sprintf("failed to create table: %v", err))
	}
}

func memorySave(args map[string]string) (string, error) {
	key, ok := args["key"]
	if !ok || key == "" {
		return "", fmt.Errorf("missing required argument: key")
	}
	value, ok := args["value"]
	if !ok {
		return "", fmt.Errorf("missing required argument: value")
	}

	_, err := db.Exec(
		`INSERT INTO memories (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save memory: %w", err)
	}

	return fmt.Sprintf("Memory '%s' saved.", key), nil
}

func memoryLoad(args map[string]string) (string, error) {
	rows, err := db.Query("SELECT key, value FROM memories ORDER BY updated_at DESC")
	if err != nil {
		return "", fmt.Errorf("failed to load memories: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, fmt.Sprintf("%s: %s", key, value))
	}

	if len(results) == 0 {
		return "No memories stored.", nil
	}
	return fmt.Sprintf("Stored memories:\n%s", strings.Join(results, "\n")), nil
}

func memoryForget(args map[string]string) (string, error) {
	key, ok := args["key"]
	if !ok || key == "" {
		return "", fmt.Errorf("missing required argument: key")
	}

	result, err := db.Exec("DELETE FROM memories WHERE key = ?", key)
	if err != nil {
		return "", fmt.Errorf("failed to forget memory: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Sprintf("No memory found with key '%s'.", key), nil
	}
	return fmt.Sprintf("Memory '%s' forgotten.", key), nil
}

func memoryList(args map[string]string) (string, error) {
	rows, err := db.Query("SELECT key FROM memories ORDER BY updated_at DESC")
	if err != nil {
		return "", fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return "No memories stored.", nil
	}
	return fmt.Sprintf("Memory keys:\n%s", strings.Join(keys, "\n")), nil
}
