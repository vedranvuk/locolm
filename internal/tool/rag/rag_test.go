package rag

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewInitializesVectorTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if _, err := New(DefaultConfig(), db); err != nil {
		t.Fatalf("New() error = %v", err)
	}
}
