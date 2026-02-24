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
	if err := memory.AppendEpisodicDB(d, "test-session", "cmd", "git status", "%1", ""); err != nil {
		t.Fatalf("AppendEpisodic failed: %v", err)
	}
	if err := memory.AppendEpisodicDB(d, "test-session", "cmd", "go build .", "%1", ""); err != nil {
		t.Fatalf("AppendEpisodic failed: %v", err)
	}
	if err := memory.AppendEpisodicDB(d, "other-session", "cmd", "npm test", "%2", ""); err != nil {
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

	memory.AppendEpisodicDB(d, "s1", "cmd", "debugging auth middleware issue", "%1", "")
	memory.AppendEpisodicDB(d, "s1", "note", "chose JWT over session cookies", "%1", "")
	memory.AppendEpisodicDB(d, "s2", "cmd", "fixed auth bug in login handler", "%2", "")
	memory.AppendEpisodicDB(d, "s2", "cmd", "running database migration", "%2", "")

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

	memory.AppendEpisodicDB(d, "s1", "cmd", "hello", "", "")
	memory.AppendEpisodicDB(d, "s2", "cmd", "world", "", "")

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

	// Append several summary entries with different claude_session_ids
	memory.AppendEpisodicDB(d, "s1", "summary", "Set up auth middleware", "", "claude-abc")
	memory.AppendEpisodicDB(d, "s1", "summary", "Fixed token refresh bug", "", "claude-abc")
	memory.AppendEpisodicDB(d, "s1", "summary", "Wrote unit tests", "", "claude-def")
	memory.AppendEpisodicDB(d, "s1", "cmd", "go test ./...", "%1", "")

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

	// Verify claude_session_id is preserved
	if len(summaries) >= 1 && summaries[0].ClaudeSessionID != "claude-def" {
		t.Errorf("expected claude_session_id 'claude-def', got '%s'", summaries[0].ClaudeSessionID)
	}
}

func TestRecentSummariesLimit(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	for i := 0; i < 15; i++ {
		memory.AppendEpisodicDB(d, "s1", "summary", "summary entry", "", "claude-abc")
	}

	summaries, err := memory.RecentSummariesDB(d, "s1", 5)
	if err != nil {
		t.Fatalf("RecentSummariesDB failed: %v", err)
	}
	if len(summaries) != 5 {
		t.Errorf("expected 5 summaries with limit, got %d", len(summaries))
	}
}

func TestEpisodicWithClaudeSessionID(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Append with claude_session_id
	if err := memory.AppendEpisodicDB(d, "s1", "pane_snapshot", "some buffer", "%5", "claude-xyz"); err != nil {
		t.Fatalf("AppendEpisodic failed: %v", err)
	}

	entries, err := memory.RecentEpisodicDB(d, "s1", 10)
	if err != nil {
		t.Fatalf("RecentEpisodic failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ClaudeSessionID != "claude-xyz" {
		t.Errorf("expected claude_session_id 'claude-xyz', got '%s'", entries[0].ClaudeSessionID)
	}
}

func TestContextFiltersPaneSnapshotsAndSummaries(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Create working state
	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	// Add various episodic entries
	memory.AppendEpisodicDB(d, "s1", "activate", "session activated", "", "")
	memory.AppendEpisodicDB(d, "s1", "pane_snapshot", "long buffer content...", "%1", "claude-abc")
	memory.AppendEpisodicDB(d, "s1", "summary", "Fixed auth bug", "", "claude-abc")
	memory.AppendEpisodicDB(d, "s1", "cmd", "go test ./...", "%1", "")
	memory.AppendEpisodicDB(d, "s1", "deactivate", "session deactivated", "", "")

	ctx, err := memory.RenderContextDB(d, "s1")
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	// pane_snapshot and summary should NOT appear in Recent Activity
	if strings.Contains(ctx, "long buffer content") {
		t.Error("pane_snapshot content should be filtered from Recent Activity")
	}

	// activate and cmd should appear
	if !strings.Contains(ctx, "session activated") {
		t.Error("activate entry should appear in Recent Activity")
	}
	if !strings.Contains(ctx, "go test") {
		t.Error("cmd entry should appear in Recent Activity")
	}
}

func TestSessionHistoryRendering(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Create working state
	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	// Add multiple summaries
	memory.AppendEpisodicDB(d, "s1", "summary", "First summary", "", "claude-abc")
	memory.AppendEpisodicDB(d, "s1", "summary", "Second summary", "", "claude-abc")
	memory.AppendEpisodicDB(d, "s1", "summary", "Third summary (most recent)", "", "claude-abc")

	ctx, err := memory.RenderContextDB(d, "s1")
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	// Session History section should exist (>1 summary)
	if !strings.Contains(ctx, "## Session History") {
		t.Error("expected Session History section")
	}

	// Most recent summary is skipped (same as last_summary)
	// "First summary" and "Second summary" should appear
	if !strings.Contains(ctx, "First summary") {
		t.Error("expected First summary in Session History")
	}
	if !strings.Contains(ctx, "Second summary") {
		t.Error("expected Second summary in Session History")
	}
}

func TestSessionHistoryGroupsByAgent(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	// Add summaries from multiple agents
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent 1 work", "", "claude-agent1")
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent 2 work", "", "claude-agent2")
	memory.AppendEpisodicDB(d, "s1", "summary", "More agent 1 work", "", "claude-agent1")
	memory.AppendEpisodicDB(d, "s1", "summary", "Latest (skipped)", "", "claude-agent1")

	ctx, err := memory.RenderContextDB(d, "s1")
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	// With multiple agents, should have grouping headers
	if !strings.Contains(ctx, "### Agent (") {
		t.Error("expected agent grouping headers for multi-agent Session History")
	}
}

func TestPerAgentContextInjection(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo", LastSummary: "session-level summary"}
	memory.UpsertWorkingDB(d, w)

	// 3 agents with different summaries
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent A: built auth system", "", "agent-a")
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent B: wrote unit tests", "", "agent-b")
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent C: set up CI pipeline", "", "agent-c")
	// Each agent has older history too
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent A: latest auth work", "", "agent-a")
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent B: latest test work", "", "agent-b")
	memory.AppendEpisodicDB(d, "s1", "summary", "Agent C: latest CI work", "", "agent-c")

	// Filter to agent-a: should see only agent-a's summary
	ctx, err := memory.RenderContextDBForAgent(d, "s1", "agent-a")
	if err != nil {
		t.Fatalf("RenderContextDBForAgent failed: %v", err)
	}

	// Last Summary should be agent-a specific, not session-level
	if strings.Contains(ctx, "session-level summary") {
		t.Error("per-agent context should use agent-specific summary, not session-level")
	}
	if !strings.Contains(ctx, "Agent A: latest auth work") {
		t.Error("expected agent-a's latest summary as Last Summary")
	}

	// Session History should only contain agent-a entries
	if strings.Contains(ctx, "Agent B") {
		t.Error("agent-a context should not contain agent-b summaries")
	}
	if strings.Contains(ctx, "Agent C") {
		t.Error("agent-a context should not contain agent-c summaries")
	}
	if !strings.Contains(ctx, "Agent A: built auth system") {
		t.Error("agent-a context should contain its own history")
	}

	// Filter to agent-b: should see only agent-b
	ctx2, _ := memory.RenderContextDBForAgent(d, "s1", "agent-b")
	if strings.Contains(ctx2, "Agent A") {
		t.Error("agent-b context should not contain agent-a summaries")
	}
	if !strings.Contains(ctx2, "Agent B: latest test work") {
		t.Error("expected agent-b's latest summary")
	}
}
