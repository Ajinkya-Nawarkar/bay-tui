package search

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/tui/styles"
)

// View renders the search screen.
func (m Model) View() string {
	w := m.width
	if w < 40 {
		w = 80
	}
	h := m.height
	if h < 10 {
		h = 24
	}

	pad := "  "
	innerW := w - 4 // border + padding

	// Header
	header := pad + styles.Title.Render("search sessions")

	// Input
	inputRow := pad + "\U0001F50D " + m.input.View()

	// Results
	maxResults := h - 10 // leave room for header, input, detail, help
	if maxResults < 3 {
		maxResults = 3
	}

	var resultRows []string
	if len(m.filtered) == 0 {
		resultRows = append(resultRows, pad+styles.NoSessions.Render("no matches"))
	} else {
		// Scroll window
		start := 0
		if m.cursor >= maxResults {
			start = m.cursor - maxResults + 1
		}
		end := start + maxResults
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			e := m.filtered[i]
			row := m.renderSessionRow(e, i == m.cursor, innerW)
			resultRows = append(resultRows, pad+row)
		}

		// Scroll indicator
		if end < len(m.filtered) {
			remaining := len(m.filtered) - end
			resultRows = append(resultRows, pad+styles.HelpBar.Render(fmt.Sprintf("  ... %d more below", remaining)))
		}
	}

	// Detail panel for selected session
	var detailRows []string
	if m.cursor < len(m.filtered) {
		e := m.filtered[m.cursor]
		sep := pad + styles.HelpBar.Render(strings.Repeat("─", min(innerW-4, 50)))
		detailRows = append(detailRows, sep)

		if e.Note != "" {
			detailRows = append(detailRows, pad+styles.HelpBar.Render("note    ")+styles.CollapsedNote.Render(e.Note))
		}
		if e.PaneInfo != "" {
			detailRows = append(detailRows, pad+styles.HelpBar.Render("panes   ")+styles.SessionName.Render(e.PaneInfo))
		}
	}

	// Help bar
	var helpText string
	if len(m.filtered) == 0 {
		helpText = "esc back"
	} else if len(m.filtered) == 1 {
		helpText = fmt.Sprintf("enter to switch to %s", m.filtered[0].Session.Name)
	} else {
		helpText = fmt.Sprintf("%d matches  \u2191\u2193 navigate  enter switch  esc back", len(m.filtered))
	}
	helpRow := pad + styles.HelpBar.Render(helpText)

	// Assemble
	lines := []string{header, "", inputRow, ""}
	lines = append(lines, resultRows...)
	lines = append(lines, "")
	lines = append(lines, detailRows...)

	// Pad to fill height, then help at bottom
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	lines = append(lines, helpRow)

	return strings.Join(lines, "\n")
}

func (m Model) renderSessionRow(e enrichedSession, selected bool, maxW int) string {
	cursor := "  "
	if selected {
		cursor = styles.GridSessionSelected.Render("\u25b8 ")
	}

	// repo/name
	label := e.Session.Repo + "/" + stripRepoPrefix(e.Session.Name, e.Session.Repo)
	if selected {
		label = styles.GridSessionSelected.Render(label)
	} else {
		label = styles.GridSessionItem.Render(label)
	}

	// Branch
	branch := ""
	if e.Branch != "" {
		branch = styles.SessionName.Render("\u2443 " + e.Branch)
	}

	// Agent indicator
	agent := agentIndicator(e)

	// Time
	t := e.Session.LastActiveAt
	if t.IsZero() {
		t = e.Session.CreatedAt
	}
	age := styles.HelpBar.Render(relativeTime(t))

	// Diff
	diff := renderDiffCompact(e.Diff)

	// Assemble with spacing
	parts := []string{cursor + label}
	if branch != "" {
		parts = append(parts, branch)
	}
	if agent != "" {
		parts = append(parts, agent)
	}
	if age != "" {
		parts = append(parts, age)
	}
	if diff != "" {
		parts = append(parts, diff)
	}
	return strings.Join(parts, "  ")
}

func agentIndicator(e enrichedSession) string {
	if !e.HasAgent {
		return styles.HelpBar.Render("\u25c7")
	}
	if e.AgentActive {
		return styles.NoteText.Render("\u25c6 active")
	}
	return styles.SuccessText.Render("\u25c6 idle")
}

func renderDiffCompact(d diffSummary) string {
	if d.Clean {
		return styles.SuccessText.Render("\u2713 clean")
	}
	if d.Files == 0 && d.Insertions == 0 && d.Deletions == 0 {
		return ""
	}
	return styles.NoteText.Render(fmt.Sprintf("\u00b1%d", d.Files)) + " " +
		styles.SuccessText.Render(fmt.Sprintf("+%d", d.Insertions)) + " " +
		styles.ErrorText.Render(fmt.Sprintf("-%d", d.Deletions))
}

var CollapsedNote = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F97316")).
	Italic(true)

func stripRepoPrefix(name, repo string) string {
	prefix := strings.ToLower(repo) + "-"
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, prefix) && len(name) > len(prefix) {
		return name[len(prefix):]
	}
	return name
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
