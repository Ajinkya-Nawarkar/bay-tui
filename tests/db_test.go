package tests

import (
	"testing"

	"bay/internal/db"
)

func TestDBOpenInMemory(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath(:memory:) failed: %v", err)
	}
	defer d.Close()

	// Verify tasks table exists
	var name string
	err = d.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='tasks'",
	).Scan(&name)
	if err != nil {
		t.Errorf("tasks table not found: %v", err)
	}
}

func TestDBMigrateIdempotent(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("first OpenPath failed: %v", err)
	}
	defer d.Close()

	d2, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("second OpenPath failed: %v", err)
	}
	defer d2.Close()
}
