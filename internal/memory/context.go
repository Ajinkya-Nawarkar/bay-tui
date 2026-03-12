package memory

import (
	"database/sql"
	"fmt"
	"strings"

	"bay/internal/config"
	"bay/internal/db"
)

// RenderContext queries working_state for a session and renders slim context:
// header (name + repo), tasks, summary, and optional session note.
func RenderContext(sessionID, sessionNote string) (string, error) {
	return RenderContextDB(nil, sessionID, sessionNote, 0)
}

// RenderContextDB renders slim context using the given DB (or default).
// paneTaskID is the task ID assigned to the current pane (0 = none).
func RenderContextDB(d *sql.DB, sessionID, sessionNote string, paneTaskID int) (string, error) {
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

	var b strings.Builder
	b.WriteString(renderHeader(w, sessionID))
	b.WriteString(renderTasks(d, sessionID, paneTaskID))
	b.WriteString(renderLastSummary(w))
	b.WriteString(renderSessionNote(sessionNote))

	return b.String(), nil
}


// renderHeader builds a minimal header: session name + repo only.
func renderHeader(w *WorkingState, sessionID string) string {
	var b strings.Builder
	b.WriteString("# Bay Session Context\n")
	if w != nil {
		b.WriteString(fmt.Sprintf("> Session: %s | Repo: %s\n", w.SessionID, w.Repo))
	} else {
		b.WriteString(fmt.Sprintf("> Session: %s | No working state recorded\n", sessionID))
	}
	return b.String()
}

// renderTasks builds the tasks section from the tasks table.
// If paneTaskID > 0, marks the assigned task.
func renderTasks(d *sql.DB, sessionID string, paneTaskID int) string {
	tasks, err := ListTasksDB(d, sessionID)
	if err != nil || len(tasks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Tasks\n")

	for i, t := range tasks {
		marker := "[ ]"
		switch t.Status {
		case "done":
			marker = "[x]"
		case "doing":
			marker = "[>]"
		}

		prefix := ""
		if t.ParentID != nil {
			prefix = "  "
		}

		suffix := ""
		if paneTaskID > 0 && t.ID == int64(paneTaskID) {
			suffix = " ← assigned to this pane"
		}

		status := ""
		if t.Status != "todo" {
			status = fmt.Sprintf(" (%s)", t.Status)
		}

		b.WriteString(fmt.Sprintf("%s- %s %d. %s%s%s\n", prefix, marker, i+1, t.Title, status, suffix))
	}

	return b.String()
}

// renderLastSummary builds the last summary section.
func renderLastSummary(w *WorkingState) string {
	if w == nil || w.LastSummary == "" {
		return ""
	}
	return fmt.Sprintf("\n## Last Summary\n%s\n", w.LastSummary)
}

// renderSessionNote renders the session note if non-empty.
func renderSessionNote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return fmt.Sprintf("\n## Session Note\n%s\n", note)
}
