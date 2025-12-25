package db_test

import (
	"testing"

	"github.com/palm-arcade/votigo/internal/db"
)

func TestOpen(t *testing.T) {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer conn.Close()

	// Verify connection works
	var result int
	err = conn.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}
}

func TestMigrate(t *testing.T) {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer conn.Close()

	err = db.Migrate(conn)
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Verify categories table exists
	var name string
	err = conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='categories'").Scan(&name)
	if err != nil {
		t.Fatalf("categories table not found: %v", err)
	}
}
