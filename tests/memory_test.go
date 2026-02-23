package tests

import (
	"testing"
	"time"

	"bay/internal/db"
	"bay/internal/memory"
)

func setupTestDB(t *testing.T) *testing.T {
	t.Helper()
	return t
}

func TestEpisodicAppendAndRecent(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Append some entries
	if err := memory.AppendEpisodicDB(d, "test-session", "cmd", "git status", "%1"); err != nil {
		t.Fatalf("AppendEpisodic failed: %v", err)
	}
	if err := memory.AppendEpisodicDB(d, "test-session", "cmd", "go build .", "%1"); err != nil {
		t.Fatalf("AppendEpisodic failed: %v", err)
	}
	if err := memory.AppendEpisodicDB(d, "other-session", "cmd", "npm test", "%2"); err != nil {
		t.Fatalf("AppendEpisodic failed: %v", err)
	}

	// Query recent for test-session
	entries, err := memory.RecentEpisodicDB(d, "test-session", 10)
	if err != nil {
		t.Fatalf("RecentEpisodic failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	// Should be in reverse order (most recent first)
	if len(entries) >= 2 && entries[0].Content != "go build ." {
		t.Errorf("expected most recent first, got %s", entries[0].Content)
	}

	// Query with limit
	entries, err = memory.RecentEpisodicDB(d, "test-session", 1)
	if err != nil {
		t.Fatalf("RecentEpisodic with limit failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestEpisodicSearch(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	memory.AppendEpisodicDB(d, "s1", "cmd", "debugging auth middleware issue", "%1")
	memory.AppendEpisodicDB(d, "s1", "note", "chose JWT over session cookies", "%1")
	memory.AppendEpisodicDB(d, "s2", "cmd", "fixed auth bug in login handler", "%2")
	memory.AppendEpisodicDB(d, "s2", "cmd", "running database migration", "%2")

	// Search across all sessions
	results, err := memory.SearchEpisodicDB(d, "auth", "")
	if err != nil {
		t.Fatalf("SearchEpisodic failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'auth', got %d", len(results))
	}

	// Search within one session
	results, err = memory.SearchEpisodicDB(d, "auth", "s1")
	if err != nil {
		t.Fatalf("SearchEpisodic filtered failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'auth' in s1, got %d", len(results))
	}

	// Search for term that only exists in one session
	results, err = memory.SearchEpisodicDB(d, "JWT", "")
	if err != nil {
		t.Fatalf("SearchEpisodic JWT failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'JWT', got %d", len(results))
	}
}

func TestEpisodicDelete(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	memory.AppendEpisodicDB(d, "s1", "cmd", "hello", "")
	memory.AppendEpisodicDB(d, "s2", "cmd", "world", "")

	if err := memory.DeleteSessionEpisodicDB(d, "s1"); err != nil {
		t.Fatalf("DeleteSessionEpisodic failed: %v", err)
	}

	entries, _ := memory.RecentEpisodicDB(d, "s1", 10)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}

	entries, _ = memory.RecentEpisodicDB(d, "s2", 10)
	if len(entries) != 1 {
		t.Errorf("expected s2 entries preserved, got %d", len(entries))
	}
}

func TestWorkingStateUpsertAndGet(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	now := time.Now()
	w := &memory.WorkingState{
		SessionID:    "test-session",
		Repo:         "bay",
		WorktreePath: "/home/user/.bay/worktrees/bay/feature",
		GitBranch:    "feature",
		CurrentTask:  "implementing auth",
		ActiveSince:  &now,
	}

	if err := memory.UpsertWorkingDB(d, w); err != nil {
		t.Fatalf("UpsertWorking failed: %v", err)
	}

	loaded, err := memory.GetWorkingDB(d, "test-session")
	if err != nil {
		t.Fatalf("GetWorking failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected working state, got nil")
	}
	if loaded.Repo != "bay" {
		t.Errorf("expected repo 'bay', got '%s'", loaded.Repo)
	}
	if loaded.GitBranch != "feature" {
		t.Errorf("expected branch 'feature', got '%s'", loaded.GitBranch)
	}
	if loaded.CurrentTask != "implementing auth" {
		t.Errorf("expected task 'implementing auth', got '%s'", loaded.CurrentTask)
	}

	// Update via upsert
	w.CurrentTask = "writing tests"
	if err := memory.UpsertWorkingDB(d, w); err != nil {
		t.Fatalf("UpsertWorking update failed: %v", err)
	}

	loaded, err = memory.GetWorkingDB(d, "test-session")
	if err != nil {
		t.Fatalf("GetWorking after update failed: %v", err)
	}
	if loaded.CurrentTask != "writing tests" {
		t.Errorf("expected updated task, got '%s'", loaded.CurrentTask)
	}
}

func TestWorkingStateNotFound(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	loaded, err := memory.GetWorkingDB(d, "nonexistent")
	if err != nil {
		t.Fatalf("GetWorking should not error for missing session: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestSetTask(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// First create the row
	w := &memory.WorkingState{SessionID: "s1", Repo: "bay"}
	memory.UpsertWorkingDB(d, w)

	if err := memory.SetTaskDB(d, "s1", "fix bug #42"); err != nil {
		t.Fatalf("SetTask failed: %v", err)
	}

	loaded, _ := memory.GetWorkingDB(d, "s1")
	if loaded.CurrentTask != "fix bug #42" {
		t.Errorf("expected task 'fix bug #42', got '%s'", loaded.CurrentTask)
	}
}

func TestSetSummary(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "s1", Repo: "bay"}
	memory.UpsertWorkingDB(d, w)

	if err := memory.SetSummaryDB(d, "s1", "Fixed auth middleware"); err != nil {
		t.Fatalf("SetSummary failed: %v", err)
	}

	loaded, _ := memory.GetWorkingDB(d, "s1")
	if loaded.LastSummary != "Fixed auth middleware" {
		t.Errorf("expected summary, got '%s'", loaded.LastSummary)
	}
}

func TestWorkingDelete(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "s1", Repo: "bay"}
	memory.UpsertWorkingDB(d, w)

	if err := memory.DeleteWorkingDB(d, "s1"); err != nil {
		t.Fatalf("DeleteWorking failed: %v", err)
	}

	loaded, _ := memory.GetWorkingDB(d, "s1")
	if loaded != nil {
		t.Error("expected nil after delete")
	}
}

func TestWorkingRename(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "old-name", Repo: "bay", CurrentTask: "testing"}
	memory.UpsertWorkingDB(d, w)

	if err := memory.RenameWorkingDB(d, "old-name", "new-name"); err != nil {
		t.Fatalf("RenameWorking failed: %v", err)
	}

	old, _ := memory.GetWorkingDB(d, "old-name")
	if old != nil {
		t.Error("old name should not exist after rename")
	}

	new, _ := memory.GetWorkingDB(d, "new-name")
	if new == nil {
		t.Fatal("new name should exist after rename")
	}
	if new.CurrentTask != "testing" {
		t.Errorf("expected task preserved after rename, got '%s'", new.CurrentTask)
	}
}
