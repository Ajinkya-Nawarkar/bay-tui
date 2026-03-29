package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

// RenderContext renders session purpose + checklist for context injection.
func RenderContext(sessionID, repo, branch, purpose string) (string, error) {
	return RenderContextDB(nil, sessionID, repo, branch, purpose)
}

// RenderContextDB renders session purpose + checklist using the given DB (or default).
func RenderContextDB(d *sql.DB, sessionID, repo, branch, purpose string) (string, error) {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Session: %s\n", sessionID))
	info := fmt.Sprintf("> Repo: %s", repo)
	if branch != "" {
		info += fmt.Sprintf(" | Branch: %s", branch)
	}
	b.WriteString(info + "\n")

	// Purpose
	purpose = strings.TrimSpace(purpose)
	if purpose != "" {
		b.WriteString(fmt.Sprintf("\n## Purpose\n%s\n", purpose))
	}

	// Checklist
	b.WriteString(renderChecklist(d, sessionID))

	return b.String(), nil
}

// renderChecklist builds the flat checklist from the tasks table.
func renderChecklist(d *sql.DB, sessionID string) string {
	tasks, err := ListTasksDB(d, sessionID)
	if err != nil || len(tasks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Checklist\n")

	for _, t := range tasks {
		marker := "[ ]"
		if t.Status == "done" {
			marker = "[x]"
		}
		b.WriteString(fmt.Sprintf("- %s %s\n", marker, t.Title))
	}

	return b.String()
}
