package memory

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"bay/internal/config"
	"bay/internal/db"
)

// RenderContext queries working_state + recent episodic for a session,
// compiles everything into a structured markdown block, and returns it as a string.
func RenderContext(sessionID string) (string, error) {
	return RenderContextDB(nil, sessionID)
}

// RenderContextDB renders context using the given DB (or default).
func RenderContextDB(d *sql.DB, sessionID string) (string, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return "", fmt.Errorf("opening db: %w", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		defaultCfg := config.DefaultConfig()
		cfg = defaultCfg
	}

	if !cfg.Memory.Enabled || !cfg.Memory.ContextInjection {
		return "", nil
	}

	// Get working state
	w, err := GetWorkingDB(d, sessionID)
	if err != nil {
		return "", fmt.Errorf("getting working state: %w", err)
	}

	// Build always-included sections
	header := renderHeader(w, sessionID)
	task := renderTask(w)
	summary := renderLastSummary(w)

	// Determine budget
	budget := cfg.Memory.ContextBudget
	if budget <= 0 {
		budget = math.MaxInt
	}

	remaining := budget - len(header) - len(task) - len(summary)

	// Build budget-aware sections
	history := ""
	if remaining > 200 {
		history = renderSessionHistory(d, sessionID, remaining)
		remaining -= len(history)
	}

	activity := ""
	if remaining > 200 {
		activity = renderRecentActivity(d, sessionID, remaining)
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString(task)
	b.WriteString(summary)
	b.WriteString(history)
	b.WriteString(activity)

	return b.String(), nil
}

// renderHeader builds the always-included header section.
func renderHeader(w *WorkingState, sessionID string) string {
	var b strings.Builder
	b.WriteString("# Bay Session Context\n")
	if w != nil {
		lastActive := w.LastUpdated.Format(time.RFC822)
		b.WriteString(fmt.Sprintf("> Session: %s | Repo: %s", w.SessionID, w.Repo))
		if w.GitBranch != "" {
			b.WriteString(fmt.Sprintf(" | Branch: %s", w.GitBranch))
		}
		b.WriteString(fmt.Sprintf(" | Last active: %s\n", lastActive))
	} else {
		b.WriteString(fmt.Sprintf("> Session: %s | No working state recorded\n", sessionID))
	}
	return b.String()
}

// renderTask builds the current task section (always included).
func renderTask(w *WorkingState) string {
	if w == nil || w.CurrentTask == "" {
		return ""
	}
	return fmt.Sprintf("\n## Where You Left Off\n**Current Task**: %s\n", w.CurrentTask)
}

// renderLastSummary builds the last summary section (always included).
func renderLastSummary(w *WorkingState) string {
	if w == nil || w.LastSummary == "" {
		return ""
	}
	return fmt.Sprintf("\n## Last Summary\n%s\n", w.LastSummary)
}

// renderSessionHistory builds the session history section, respecting budget.
func renderSessionHistory(d *sql.DB, sessionID string, budget int) string {
	summaries, err := RecentSummariesDB(d, sessionID, 30)
	if err != nil || len(summaries) <= 1 {
		return ""
	}

	// Skip the most recent one (same as last_summary)
	summaries = summaries[1:]
	if len(summaries) == 0 {
		return ""
	}

	var lines []string
	for _, e := range summaries {
		ts := e.Timestamp.Format("02 Jan 15:04")
		lines = append(lines, fmt.Sprintf("- [%s] %s\n", ts, e.Content))
	}
	return renderBudgetedSection("\n## Session History\n", lines, budget)
}

// renderRecentActivity builds the recent activity section, respecting budget.
func renderRecentActivity(d *sql.DB, sessionID string, budget int) string {
	entries, err := RecentEpisodicDB(d, sessionID, 20)
	if err != nil || len(entries) == 0 {
		return ""
	}

	var lines []string
	for _, e := range entries {
		switch e.Type {
		case "pane_snapshot", "summary":
			continue
		default:
			ts := e.Timestamp.Format("15:04")
			lines = append(lines, fmt.Sprintf("- [%s] (%s) %s\n", ts, e.Type, e.Content))
		}
	}
	if len(lines) > 10 {
		lines = lines[:10]
	}
	return renderBudgetedSection("\n## Recent Activity\n", lines, budget)
}

// renderBudgetedSection writes a section header followed by as many lines as fit within budget.
func renderBudgetedSection(header string, lines []string, budget int) string {
	if len(lines) == 0 {
		return ""
	}

	remaining := budget - len(header)
	if remaining <= 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(header)
	wrote := false
	for _, line := range lines {
		if remaining-len(line) < 0 {
			break
		}
		b.WriteString(line)
		remaining -= len(line)
		wrote = true
	}

	if !wrote {
		return ""
	}
	return b.String()
}
