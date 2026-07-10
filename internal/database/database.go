package database

import (
	"database/sql"
	"fmt"
)

type Config struct {
	// DSN is the database data source name.
	DSN string
}

// DefaultConfig returns the default database config.
func DefaultConfig() *Config {
	return &Config{
		DSN: "locolm.db",
	}
}

// Open opens a database given config.
func Open(config *Config) (*sql.DB, error) {
	if config == nil {
		config = DefaultConfig()
	}
	var db, err = sql.Open("sqlite", config.DSN+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return db, nil
}
