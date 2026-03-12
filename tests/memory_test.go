package tests

import (
	"strings"
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

func TestRollingSummaries(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Append several summary entries
	memory.AppendEpisodicDB(d, "s1", "summary", "Set up auth middleware", "")
	memory.AppendEpisodicDB(d, "s1", "summary", "Fixed token refresh bug", "")
	memory.AppendEpisodicDB(d, "s1", "summary", "Wrote unit tests", "")
	memory.AppendEpisodicDB(d, "s1", "cmd", "go test ./...", "%1")

	// RecentSummariesDB should return only summary entries
	summaries, err := memory.RecentSummariesDB(d, "s1", 10)
	if err != nil {
		t.Fatalf("RecentSummariesDB failed: %v", err)
	}
	if len(summaries) != 3 {
		t.Errorf("expected 3 summaries, got %d", len(summaries))
	}

	// Most recent first
	if len(summaries) >= 1 && summaries[0].Content != "Wrote unit tests" {
		t.Errorf("expected newest first, got '%s'", summaries[0].Content)
	}
}

func TestRecentSummariesLimit(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	for i := 0; i < 15; i++ {
		memory.AppendEpisodicDB(d, "s1", "summary", "summary entry", "")
	}

	summaries, err := memory.RecentSummariesDB(d, "s1", 5)
	if err != nil {
		t.Fatalf("RecentSummariesDB failed: %v", err)
	}
	if len(summaries) != 5 {
		t.Errorf("expected 5 summaries with limit, got %d", len(summaries))
	}
}

func TestSlimContext(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Create working state with summary
	w := &memory.WorkingState{
		SessionID:   "s1",
		Repo:        "myrepo",
		LastSummary: "Set up middleware and wrote tests.",
	}
	memory.UpsertWorkingDB(d, w)

	// Create a task in the tasks table
	memory.CreateTaskDB(d, "s1", "implement auth", nil)

	// Add episodic entries that should NOT appear in slim context
	memory.AppendEpisodicDB(d, "s1", "activate", "session activated", "")
	memory.AppendEpisodicDB(d, "s1", "cmd", "go test ./...", "%1")
	memory.AppendEpisodicDB(d, "s1", "summary", "First summary", "")
	memory.AppendEpisodicDB(d, "s1", "summary", "Second summary", "")

	ctx, err := memory.RenderContextDB(d, "s1", "working on auth feature", 0)
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	// Should contain: header, tasks, summary, note
	if !strings.Contains(ctx, "Session: s1") {
		t.Error("expected session name in header")
	}
	if !strings.Contains(ctx, "Repo: myrepo") {
		t.Error("expected repo in header")
	}
	if !strings.Contains(ctx, "implement auth") {
		t.Error("expected task in context")
	}
	if !strings.Contains(ctx, "## Tasks") {
		t.Error("expected Tasks section in context")
	}
	if !strings.Contains(ctx, "Set up middleware") {
		t.Error("expected summary in context")
	}
	if !strings.Contains(ctx, "## Session Note") {
		t.Error("expected session note section")
	}
	if !strings.Contains(ctx, "working on auth feature") {
		t.Error("expected note content")
	}

	// Should NOT contain: history, activity, branch, timestamps
	if strings.Contains(ctx, "## Session History") {
		t.Error("slim context should not contain Session History")
	}
	if strings.Contains(ctx, "## Recent Activity") {
		t.Error("slim context should not contain Recent Activity")
	}
	if strings.Contains(ctx, "Branch:") {
		t.Error("slim context should not contain Branch")
	}
	if strings.Contains(ctx, "Last active:") {
		t.Error("slim context should not contain Last active timestamp")
	}
}

func TestSlimContextNoNote(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{
		SessionID: "s1",
		Repo:      "myrepo",
	}
	memory.UpsertWorkingDB(d, w)

	// Create a task in the tasks table
	memory.CreateTaskDB(d, "s1", "fix bug", nil)

	ctx, err := memory.RenderContextDB(d, "s1", "", 0)
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	// Should NOT contain note section when note is empty
	if strings.Contains(ctx, "## Session Note") {
		t.Error("note section should be omitted when empty")
	}

	// Should still contain task
	if !strings.Contains(ctx, "fix bug") {
		t.Error("expected task in context")
	}
}

func TestIsNonTrivialBuffer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"empty string", "", false},
		{"whitespace only", "   \n\n  \n   ", false},
		{"shell prompts only", "➜ tests git:(tests)\n$ \n% \n", false},
		{"prompts with one command", "➜ tests git:(tests) ls\nfile1.go\n", false},
		{"real terminal output", "➜ tests git:(tests) go test ./...\nok  bay/tests 0.5s\n--- PASS: TestFoo\nPASS\ncoverage: 80%\n", true},
		{"multiple meaningful lines", "line one\nline two\nline three\n", true},
		{"bare prompts mixed", "$ \n% \n> \nhello world\ngoodbye\nfoo bar\n", true},
		{"only trivial commands", "clear\nexit\n\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := memory.IsNonTrivialBuffer(tt.input)
			if got != tt.expect {
				t.Errorf("IsNonTrivialBuffer(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestIsLowValueSummary(t *testing.T) {
	// Test via ShouldUpdateSummary since isLowValueSummary is unexported
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// No existing session — low-value summary should still be accepted (something > nothing)
	if !memory.ShouldUpdateSummary(d, "nonexistent", "No work was performed during this session.") {
		t.Error("expected low-value summary accepted when no existing state")
	}

	// Create session with a good summary
	w := &memory.WorkingState{SessionID: "s1", Repo: "bay", LastSummary: "Fixed auth middleware and ran tests."}
	memory.UpsertWorkingDB(d, w)

	// Low-value summary should NOT overwrite good summary
	if memory.ShouldUpdateSummary(d, "s1", "No work was performed during this session.") {
		t.Error("expected low-value summary rejected when good summary exists")
	}
	if memory.ShouldUpdateSummary(d, "s1", "no meaningful activity was observed") {
		t.Error("expected 'no meaningful activity' rejected")
	}
	if memory.ShouldUpdateSummary(d, "s1", "The session was idle.") {
		t.Error("expected 'idle' summary rejected")
	}

	// Substantive summary should overwrite
	if !memory.ShouldUpdateSummary(d, "s1", "The user modified auth.go and ran tests.") {
		t.Error("expected substantive summary accepted")
	}

	// Empty existing summary — low-value should be accepted
	w2 := &memory.WorkingState{SessionID: "s2", Repo: "bay"}
	memory.UpsertWorkingDB(d, w2)

	if !memory.ShouldUpdateSummary(d, "s2", "No work was performed.") {
		t.Error("expected low-value summary accepted when existing summary is empty")
	}
}

func TestCleanStalePendingSummaries(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Insert a fresh row
	d.Exec(`INSERT INTO pending_summaries (session_id, raw_buffer, retry_count) VALUES (?, ?, ?)`,
		"fresh", "buffer1", 0)

	// Insert a row with exhausted retries
	d.Exec(`INSERT INTO pending_summaries (session_id, raw_buffer, retry_count) VALUES (?, ?, ?)`,
		"retried", "buffer2", 3)

	// Insert an old row (simulate by setting created_at in the past)
	d.Exec(`INSERT INTO pending_summaries (session_id, raw_buffer, retry_count, created_at) VALUES (?, ?, ?, datetime('now', '-2 hours'))`,
		"old", "buffer3", 0)

	// Run cleanup
	memory.CleanStalePendingSummariesDB(d)

	// Check counts
	var count int
	d.QueryRow(`SELECT COUNT(*) FROM pending_summaries WHERE session_id = 'fresh'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected fresh row preserved, got count %d", count)
	}

	d.QueryRow(`SELECT COUNT(*) FROM pending_summaries WHERE session_id = 'retried'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected retried row deleted, got count %d", count)
	}

	d.QueryRow(`SELECT COUNT(*) FROM pending_summaries WHERE session_id = 'old'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected old row deleted, got count %d", count)
	}
}

func TestSlimContextMinimal(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Working state with no task, no summary
	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	ctx, err := memory.RenderContextDB(d, "s1", "", 0)
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	// Should just have the header
	if !strings.Contains(ctx, "# Bay Session Context") {
		t.Error("expected header")
	}
	if !strings.Contains(ctx, "Repo: myrepo") {
		t.Error("expected repo in header")
	}

	// Should NOT have any optional sections
	if strings.Contains(ctx, "## Where You Left Off") {
		t.Error("task section should be absent when no task set")
	}
	if strings.Contains(ctx, "## Last Summary") {
		t.Error("summary section should be absent when no summary")
	}
}
