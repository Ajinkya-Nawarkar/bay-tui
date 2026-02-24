package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"bay/internal/config"
	"bay/internal/db"
	"bay/internal/rules"
)

// RenderContext queries working_state + rules + recent episodic for a session,
// compiles everything into a structured markdown block, and returns it as a string.
func RenderContext(sessionID string) (string, error) {
	return RenderContextDBForAgent(nil, sessionID, "")
}

// RenderContextForAgent renders context filtered to a specific prior agent's summaries.
// If priorAgentID is empty, renders all summaries (normal mode).
func RenderContextForAgent(sessionID, priorAgentID string) (string, error) {
	return RenderContextDBForAgent(nil, sessionID, priorAgentID)
}

// RenderContextDB renders context using the given DB (or default).
func RenderContextDB(d *sql.DB, sessionID string) (string, error) {
	return RenderContextDBForAgent(d, sessionID, "")
}

// RenderContextDBForAgent renders context, optionally filtered to a prior agent's summaries.
func RenderContextDBForAgent(d *sql.DB, sessionID, priorAgentID string) (string, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return "", fmt.Errorf("opening db: %w", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		// Use defaults if config can't be loaded
		defaultCfg := config.DefaultConfig()
		cfg = defaultCfg
	}

	if !cfg.Memory.Enabled || !cfg.Memory.ContextInjection {
		return "", nil
	}

	var b strings.Builder

	// Get working state
	w, err := GetWorkingDB(d, sessionID)
	if err != nil {
		return "", fmt.Errorf("getting working state: %w", err)
	}

	b.WriteString("# Bay Session Context\n")
	if w != nil {
		lastActive := w.LastUpdated.Format(time.RFC822)
		b.WriteString(fmt.Sprintf("> Session: %s | Repo: %s", w.SessionID, w.Repo))
		if w.GitBranch != "" {
			b.WriteString(fmt.Sprintf(" | Branch: %s", w.GitBranch))
		}
		b.WriteString(fmt.Sprintf(" | Last active: %s\n", lastActive))

		// Current task
		if w.CurrentTask != "" {
			b.WriteString("\n## Where You Left Off\n")
			b.WriteString(fmt.Sprintf("**Current Task**: %s\n", w.CurrentTask))
		}

		// Last summary — use per-agent summary if filtering to a specific agent
		if priorAgentID != "" {
			agentSummary := latestAgentSummary(d, sessionID, priorAgentID)
			if agentSummary != "" {
				b.WriteString("\n## Last Summary\n")
				b.WriteString(agentSummary + "\n")
			}
		} else if w.LastSummary != "" {
			b.WriteString("\n## Last Summary\n")
			b.WriteString(w.LastSummary + "\n")
		}
	} else {
		b.WriteString(fmt.Sprintf("> Session: %s | No working state recorded\n", sessionID))
	}

	// Session History — rolling summaries (filtered to specific agent on cold boot)
	renderSessionHistory(&b, d, sessionID, priorAgentID)

	// Recent episodic entries (filtered: skip pane_snapshot and summary)
	entries, err := RecentEpisodicDB(d, sessionID, 20)
	if err == nil && len(entries) > 0 {
		var filtered []EpisodicEntry
		for _, e := range entries {
			switch e.Type {
			case "pane_snapshot", "summary":
				continue
			default:
				filtered = append(filtered, e)
			}
		}
		if len(filtered) > 10 {
			filtered = filtered[:10]
		}
		if len(filtered) > 0 {
			b.WriteString("\n## Recent Activity\n")
			for _, e := range filtered {
				ts := e.Timestamp.Format("15:04")
				b.WriteString(fmt.Sprintf("- [%s] (%s) %s\n", ts, e.Type, e.Content))
			}
		}
	}

	// Sibling sessions (same repo)
	if w != nil && cfg.Memory.SiblingContext {
		siblings, err := SiblingActivityDB(d, sessionID, w.Repo, 3)
		if err == nil && len(siblings) > 0 {
			b.WriteString("\n## Sibling Sessions (same repo)\n")
			for _, s := range siblings {
				b.WriteString(fmt.Sprintf("### %s", s.Session))
				if s.Branch != "" {
					b.WriteString(fmt.Sprintf(" (%s)", s.Branch))
				}
				b.WriteString("\n")
				if s.LastSummary != "" {
					b.WriteString(s.LastSummary + "\n")
				}
			}
		}
	}

	// Applicable rules
	if w != nil && cfg.Memory.RulesInjection {
		repoName := w.Repo
		activeRules, err := rules.ActiveRulesDB(d, repoName)
		if err == nil && len(activeRules) > 0 {
			b.WriteString("\n## Applicable Rules\n")
			for _, r := range activeRules {
				content, err := rules.ReadContent(r)
				if err != nil {
					continue
				}
				b.WriteString(fmt.Sprintf("### %s\n", r.Name))
				b.WriteString(content + "\n")
			}
		}
	}

	return b.String(), nil
}

// latestAgentSummary returns the most recent summary for a specific claude_session_id.
func latestAgentSummary(d *sql.DB, sessionID, claudeSessionID string) string {
	var content string
	err := d.QueryRow(
		`SELECT content FROM episodic
		WHERE session_id = ? AND claude_session_id = ? AND type = 'summary'
		ORDER BY id DESC LIMIT 1`,
		sessionID, claudeSessionID,
	).Scan(&content)
	if err != nil {
		return ""
	}
	return content
}

// renderSessionHistory queries rolling summaries and renders the "Session History" section.
// If priorAgentID is set, only shows that agent's summaries (cold boot per-agent injection).
// Otherwise groups by claude_session_id for multi-agent views.
func renderSessionHistory(b *strings.Builder, d *sql.DB, sessionID, priorAgentID string) {
	summaries, err := RecentSummariesDB(d, sessionID, 30)
	if err != nil || len(summaries) <= 1 {
		return
	}

	// Skip the most recent one (same as last_summary)
	summaries = summaries[1:]
	if len(summaries) == 0 {
		return
	}

	// If filtering to a specific agent, only show that agent's summaries
	if priorAgentID != "" {
		var filtered []EpisodicEntry
		for _, s := range summaries {
			if s.ClaudeSessionID == priorAgentID {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			return
		}
		b.WriteString("\n## Session History\n")
		for _, e := range filtered {
			ts := e.Timestamp.Format("02 Jan 15:04")
			b.WriteString(fmt.Sprintf("- [%s] %s\n", ts, e.Content))
		}
		return
	}

	// Group by claude_session_id
	groupOrder := []string{}
	groupMap := map[string][]EpisodicEntry{}

	for _, s := range summaries {
		key := s.ClaudeSessionID
		if _, exists := groupMap[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groupMap[key] = append(groupMap[key], s)
	}

	b.WriteString("\n## Session History\n")

	multiGroup := len(groupOrder) > 1

	for _, key := range groupOrder {
		entries := groupMap[key]
		if multiGroup && key != "" {
			// Show short ID for grouping
			shortID := key
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			b.WriteString(fmt.Sprintf("### Agent (%s)\n", shortID))
		}

		for _, e := range entries {
			ts := e.Timestamp.Format("02 Jan 15:04")
			b.WriteString(fmt.Sprintf("- [%s] %s\n", ts, e.Content))
		}
	}
}

// SiblingActivity returns recent summaries from other sessions in the same repo.
func SiblingActivity(sessionID, repoName string, limit int) ([]SiblingContext, error) {
	return SiblingActivityDB(nil, sessionID, repoName, limit)
}

// SiblingActivityDB returns sibling activity using the given DB (or default).
func SiblingActivityDB(d *sql.DB, sessionID, repoName string, limit int) ([]SiblingContext, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	rows, err := d.Query(
		`SELECT session_id, COALESCE(git_branch, ''), COALESCE(last_summary, ''), last_updated
		FROM working_state
		WHERE repo = ? AND session_id != ? AND last_summary IS NOT NULL AND last_summary != ''
		ORDER BY last_updated DESC LIMIT ?`,
		repoName, sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var siblings []SiblingContext
	for rows.Next() {
		var s SiblingContext
		if err := rows.Scan(&s.Session, &s.Branch, &s.LastSummary, &s.LastUpdated); err != nil {
			return nil, err
		}
		siblings = append(siblings, s)
	}
	return siblings, rows.Err()
}
