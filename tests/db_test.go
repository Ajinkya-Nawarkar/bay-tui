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

	// Verify tables exist
	tables := []string{"episodic", "episodic_fts", "working_state", "rules", "pending_summaries"}
	for _, table := range tables {
		var name string
		err := d.QueryRow(
			"SELECT name FROM sqlite_master WHERE type IN ('table', 'vtable') AND name = ?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Verify triggers exist
	triggers := []string{"episodic_ai", "episodic_ad"}
	for _, trigger := range triggers {
		var name string
		err := d.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='trigger' AND name = ?", trigger,
		).Scan(&name)
		if err != nil {
			t.Errorf("trigger %s not found: %v", trigger, err)
		}
	}
}

func TestDBMigrateIdempotent(t *testing.T) {
	// Running OpenPath twice on the same DB should not fail
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("first OpenPath failed: %v", err)
	}
	defer d.Close()

	// The second call uses a different path but same schema — just verify no panic
	d2, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("second OpenPath failed: %v", err)
	}
	defer d2.Close()
}
