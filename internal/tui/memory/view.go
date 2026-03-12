package tmemory

import (
	"fmt"
	"strings"

	"bay/internal/tui/styles"
)

// View renders the memory viewer.
func (m Model) View() string {
	w := m.width
	if w < 20 {
		w = 80
	}

	var b strings.Builder

	title := styles.Title.Render("bay ctx") + "   " +
		styles.Subtitle.Render(m.sessionName)
	b.WriteString(title + "\n")

	switch m.view {
	case viewTask:
		b.WriteString("\n  " + styles.Prompt.Render("Set task: ") + m.taskInput.View())
		b.WriteString("\n\n  " + styles.HelpBar.Render("enter to save, esc to cancel"))

	case viewNote:
		b.WriteString("\n  " + styles.Prompt.Render("Add note: ") + m.noteInput.View())
		b.WriteString("\n\n  " + styles.HelpBar.Render("enter to save, esc to cancel"))

	case viewLog:
		b.WriteString(m.renderLog())
		b.WriteString("\n  " + styles.HelpBar.Render("l: overview  r: refresh  esc: back"))

	default:
		b.WriteString(m.renderOverview())
		b.WriteString("\n  " + styles.HelpBar.Render("t: set task  n: add note  l: log  r: refresh  esc: back"))
	}

	if m.statusMsg != "" {
		b.WriteString("\n  " + styles.SuccessText.Render(m.statusMsg))
	}

	content := b.String()
	box := styles.SectionBox.Width(w - 2)
	return box.Render(content)
}

func (m Model) renderOverview() string {
	var b strings.Builder

	if m.working == nil {
		b.WriteString("\n  " + styles.NoSessions.Render("No working memory recorded"))
		return b.String()
	}

	w := m.working

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Repo:     %s\n", styles.RepoName.Render(w.Repo)))
	if w.GitBranch != "" {
		b.WriteString(fmt.Sprintf("  Branch:   %s\n", w.GitBranch))
	}
	if w.WorktreePath != "" {
		b.WriteString(fmt.Sprintf("  Worktree: %s\n", styles.NoSessions.Render(w.WorktreePath)))
	}

	b.WriteString("\n")
	if w.CurrentTask != "" {
		b.WriteString(fmt.Sprintf("  Task:     %s\n", styles.SessionTabActive.Render(w.CurrentTask)))
	} else {
		b.WriteString("  Task:     " + styles.NoSessions.Render("(none — press t to set)") + "\n")
	}

	if w.LastSummary != "" {
		b.WriteString("\n  " + styles.Subtitle.Render("Last Summary:") + "\n")
		// Wrap summary lines
		lines := strings.Split(w.LastSummary, "\n")
		for _, line := range lines {
			b.WriteString("  " + line + "\n")
		}
	}

	if m.pending > 0 {
		b.WriteString(fmt.Sprintf("\n  Pending summaries: %d\n", m.pending))
	}

	b.WriteString(fmt.Sprintf("\n  Last updated: %s\n", w.LastUpdated.Format("2006-01-02 15:04:05")))

	return b.String()
}

func (m Model) renderLog() string {
	var b strings.Builder

	b.WriteString("\n  " + styles.Subtitle.Render("Recent Activity") + "\n\n")

	if len(m.entries) == 0 {
		b.WriteString("  " + styles.NoSessions.Render("No episodic entries") + "\n")
		return b.String()
	}

	// Show in chronological order (entries are stored newest-first)
	for i := len(m.entries) - 1; i >= 0; i-- {
		e := m.entries[i]
		ts := e.Timestamp.Format("15:04:05")
		content := e.Content
		if len(content) > 80 {
			content = content[:77] + "..."
		}
		typeStyle := styles.HelpKey.Render(fmt.Sprintf("%-12s", e.Type))
		b.WriteString(fmt.Sprintf("  %s %s %s\n", styles.NoSessions.Render(ts), typeStyle, content))
	}

	return b.String()
}
