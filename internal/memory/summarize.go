package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"bay/internal/db"
)

// shellPromptPatterns matches common shell prompt lines that carry no meaningful content.
var shellPromptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^[%$>#]\s*$`),                          // bare prompts
	regexp.MustCompile(`^[%$>#] `),                              // prompt + space (no command)
	regexp.MustCompile(`^➜\s+\S+\s+git:\(`),                    // oh-my-zsh git prompt
	regexp.MustCompile(`^➜\s+\S+\s*$`),                         // oh-my-zsh directory-only prompt
	regexp.MustCompile(`^\S+@\S+:[^\$]*\$\s*$`),                // user@host:path$ (empty)
	regexp.MustCompile(`^(\033\[[0-9;]*m)*[%$>#➜]\s`),          // ANSI-colored prompts
	regexp.MustCompile(`^\s*$`),                                 // blank/whitespace
	regexp.MustCompile(`^(clear|exit|logout)\s*$`),              // trivial commands
}

// IsNonTrivialBuffer returns true if the buffer contains meaningful terminal content
// beyond shell prompts, blank lines, and trivial commands.
func IsNonTrivialBuffer(raw string) bool {
	lines := strings.Split(raw, "\n")
	meaningful := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isPrompt := false
		for _, pat := range shellPromptPatterns {
			if pat.MatchString(trimmed) {
				isPrompt = true
				break
			}
		}
		if !isPrompt {
			meaningful++
		}
	}
	return meaningful >= 3
}

// lowValuePhrases are indicators that a summary describes an idle/empty session.
var lowValuePhrases = []string{
	"no work done",
	"no meaningful activity",
	"no commands were executed",
	"no files were modified",
	"no meaningful work",
	"no work was performed",
	"idle",
	"no activity",
}

// isLowValueSummary returns true if the summary describes an idle or empty session.
func isLowValueSummary(summary string) bool {
	lower := strings.ToLower(summary)
	for _, phrase := range lowValuePhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// ShouldUpdateSummary decides whether a new summary should overwrite the existing one.
// Returns true if the update should proceed.
func ShouldUpdateSummary(d *sql.DB, sessionID, newSummary string) bool {
	if !isLowValueSummary(newSummary) {
		return true // substantive summary always wins
	}
	// New summary is low-value — only write if there's no existing summary
	existing, err := GetWorkingDB(d, sessionID)
	if err != nil || existing == nil {
		return true // no existing state, something is better than nothing
	}
	return existing.LastSummary == ""
}

// SummarizeAsync saves raw buffer to DB immediately, then spawns background
// goroutine for LLM summarization. TUI remains responsive.
func SummarizeAsync(sessionID, rawBuffer, paneID string) error {
	return SummarizeAsyncDB(nil, sessionID, rawBuffer, paneID)
}

// SummarizeAsyncDB saves buffer and spawns summarization using the given DB.
func SummarizeAsyncDB(d *sql.DB, sessionID, rawBuffer, paneID string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	// Save raw buffer to episodic immediately
	AppendEpisodicDB(d, sessionID, "pane_snapshot", rawBuffer, paneID)

	// Skip LLM summarization for trivially empty buffers
	if !IsNonTrivialBuffer(rawBuffer) {
		return nil
	}

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
		processSingleSummary(d, sessionID, rawBuffer)
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

	// Clean up stale entries first
	CleanStalePendingSummariesDB(d)

	rows, err := d.Query(
		`SELECT id, session_id, raw_buffer FROM pending_summaries
		WHERE retry_count < 3 AND created_at > datetime('now', '-1 hour')
		ORDER BY created_at`)
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
			err := processSingleSummaryErr(d, item.sessionID, item.rawBuffer)
			if err != nil {
				// Increment retry count on failure
				d.Exec(`UPDATE pending_summaries SET retry_count = retry_count + 1 WHERE id = ?`, item.id)
			} else {
				d.Exec(`DELETE FROM pending_summaries WHERE id = ?`, item.id)
			}
		}(p)
	}

	return nil
}

// CleanStalePendingSummariesDB removes pending summaries that are too old or have exhausted retries.
func CleanStalePendingSummariesDB(d *sql.DB) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return
		}
	}
	d.Exec(`DELETE FROM pending_summaries WHERE retry_count >= 3 OR created_at <= datetime('now', '-1 hour')`)
}

// processSingleSummary runs LLM summarization on a raw buffer and updates working_state.
// Also appends a rolling "summary" entry to episodic.
func processSingleSummary(d *sql.DB, sessionID, rawBuffer string) {
	processSingleSummaryErr(d, sessionID, rawBuffer)
}

// processSingleSummaryErr is like processSingleSummary but returns an error.
func processSingleSummaryErr(d *sql.DB, sessionID, rawBuffer string) error {
	summary, err := summarizeBuffer(rawBuffer)
	if err != nil {
		return err
	}
	if summary == "" {
		return nil
	}

	// Protect good summaries: only update if the new summary is substantive
	// or there's no existing summary
	if ShouldUpdateSummary(d, sessionID, summary) {
		SetSummaryDB(d, sessionID, summary)
	}

	// Always append to episodic for audit trail
	AppendEpisodicDB(d, sessionID, "summary", summary, "")

	// Compact if too many summaries for this session
	compactSummaries(d, sessionID)
	return nil
}

// compactSummaries keeps at most 10 summary entries per session.
// When exceeded, the oldest 10 are condensed into one via LLM.
func compactSummaries(d *sql.DB, sessionID string) {
	var count int
	err := d.QueryRow(
		`SELECT COUNT(*) FROM episodic WHERE session_id = ? AND type = 'summary'`,
		sessionID,
	).Scan(&count)
	if err != nil || count <= 10 {
		return
	}

	// Fetch the oldest 10 summaries
	rows, err := d.Query(
		`SELECT id, content FROM episodic
		WHERE session_id = ? AND type = 'summary'
		ORDER BY id ASC LIMIT 10`,
		sessionID,
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
	AppendEpisodicDB(d, sessionID, "summary", compacted, "")
}

// summarizeBuffer sends raw text to headless LLM and returns summary.
func summarizeBuffer(raw string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--print",
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--print",
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
