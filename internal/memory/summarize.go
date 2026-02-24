package memory

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"

	"bay/internal/db"
)

// SummarizeAsync saves raw buffer to DB immediately, then spawns background
// goroutine for LLM summarization. TUI remains responsive.
func SummarizeAsync(sessionID, rawBuffer, paneID, claudeSessionID string) error {
	return SummarizeAsyncDB(nil, sessionID, rawBuffer, paneID, claudeSessionID)
}

// SummarizeAsyncDB saves buffer and spawns summarization using the given DB.
func SummarizeAsyncDB(d *sql.DB, sessionID, rawBuffer, paneID, claudeSessionID string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	// Save raw buffer to episodic immediately
	AppendEpisodicDB(d, sessionID, "pane_snapshot", rawBuffer, paneID, claudeSessionID)

	// Save to pending_summaries for async processing
	_, err := d.Exec(
		`INSERT INTO pending_summaries (session_id, raw_buffer) VALUES (?, ?)`,
		sessionID, rawBuffer,
	)
	if err != nil {
		return fmt.Errorf("saving pending summary: %w", err)
	}

	// Spawn background goroutine for LLM summarization
	go func() {
		processSingleSummary(d, sessionID, rawBuffer, claudeSessionID)
		// Clean up the pending row after processing
		d.Exec(`DELETE FROM pending_summaries WHERE session_id = ? AND raw_buffer = ?`, sessionID, rawBuffer)
	}()

	return nil
}

// ProcessPendingSummaries picks up unsummarized buffers from pending_summaries
// and runs LLM on each. Called on startup to retry failed summaries.
func ProcessPendingSummaries() error {
	return ProcessPendingSummariesDB(nil)
}

// ProcessPendingSummariesDB processes pending summaries using the given DB.
func ProcessPendingSummariesDB(d *sql.DB) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	rows, err := d.Query(`SELECT id, session_id, raw_buffer FROM pending_summaries ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pending struct {
		id        int64
		sessionID string
		rawBuffer string
	}

	var items []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.sessionID, &p.rawBuffer); err != nil {
			continue
		}
		items = append(items, p)
	}

	for _, p := range items {
		go func(item pending) {
			processSingleSummary(d, item.sessionID, item.rawBuffer, "")
			d.Exec(`DELETE FROM pending_summaries WHERE id = ?`, item.id)
		}(p)
	}

	return nil
}

// processSingleSummary runs LLM summarization on a raw buffer and updates working_state.
// Also appends a rolling "summary" entry to episodic with the claude_session_id.
func processSingleSummary(d *sql.DB, sessionID, rawBuffer, claudeSessionID string) {
	summary, err := summarizeBuffer(rawBuffer)
	if err != nil {
		return
	}
	if summary == "" {
		return
	}

	// Update working_state.last_summary (as before)
	SetSummaryDB(d, sessionID, summary)

	// Append rolling summary entry to episodic
	AppendEpisodicDB(d, sessionID, "summary", summary, "", claudeSessionID)

	// Compact if too many summaries for this (session, claude_session_id) pair
	if claudeSessionID != "" {
		compactSummaries(d, sessionID, claudeSessionID)
	}
}

// compactSummaries keeps at most 10 summary entries per (session_id, claude_session_id).
// When exceeded, the oldest 10 are condensed into one via LLM.
func compactSummaries(d *sql.DB, sessionID, claudeSessionID string) {
	var count int
	err := d.QueryRow(
		`SELECT COUNT(*) FROM episodic WHERE session_id = ? AND claude_session_id = ? AND type = 'summary'`,
		sessionID, claudeSessionID,
	).Scan(&count)
	if err != nil || count <= 10 {
		return
	}

	// Fetch the oldest 10 summaries
	rows, err := d.Query(
		`SELECT id, content FROM episodic
		WHERE session_id = ? AND claude_session_id = ? AND type = 'summary'
		ORDER BY id ASC LIMIT 10`,
		sessionID, claudeSessionID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var ids []int64
	var contents []string
	for rows.Next() {
		var id int64
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			continue
		}
		ids = append(ids, id)
		contents = append(contents, content)
	}

	if len(ids) < 2 {
		return
	}

	// Condense via LLM
	combined := strings.Join(contents, "\n---\n")
	compacted, err := compactBuffer(combined)
	if err != nil || compacted == "" {
		return
	}

	// Delete the originals
	for _, id := range ids {
		d.Exec(`DELETE FROM episodic WHERE id = ?`, id)
	}

	// Insert the compacted summary
	AppendEpisodicDB(d, sessionID, "summary", compacted, "", claudeSessionID)
}

// summarizeBuffer sends raw text to headless LLM and returns summary.
func summarizeBuffer(raw string) (string, error) {
	// Use claude CLI in headless mode
	cmd := exec.Command("claude", "--print",
		"Summarize this terminal session in 2-3 concise sentences. Focus on what was accomplished, "+
			"key decisions made, and current state. Be specific about files changed and commands run. "+
			"Output only the summary, no preamble.")
	cmd.Stdin = strings.NewReader(raw)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude summarize: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// compactBuffer sends multiple summaries to LLM for condensation.
func compactBuffer(combined string) (string, error) {
	cmd := exec.Command("claude", "--print",
		"Condense these session summaries into a single paragraph preserving key decisions, "+
			"files changed, and current state. Output only the condensed summary, no preamble.")
	cmd.Stdin = strings.NewReader(combined)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude compact: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// PendingSummaryCount returns the number of pending summaries.
func PendingSummaryCount() (int, error) {
	return PendingSummaryCountDB(nil)
}

// PendingSummaryCountDB returns pending count using the given DB.
func PendingSummaryCountDB(d *sql.DB) (int, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return 0, fmt.Errorf("opening db: %w", err)
		}
	}

	var count int
	err := d.QueryRow(`SELECT COUNT(*) FROM pending_summaries`).Scan(&count)
	return count, err
}
