package status

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/tui/styles"
)

// View renders the status dashboard.
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
	innerW := w - 4

	// Header with summary
	headerLeft := pad + styles.Title.Render("session status")
	summaryRight := m.renderSummaryColored()
	gap := innerW - lipgloss.Width(headerLeft) - lipgloss.Width(summaryRight)
	if gap < 2 {
		gap = 2
	}
	header := headerLeft + strings.Repeat(" ", gap) + summaryRight

	// Section rows
	var rows []string
	flatIdx := 0
	for _, sec := range m.sections {
		rows = append(rows, "")
		rows = append(rows, pad+styles.Subtitle.Render(sec.Label))

		for _, ss := range sec.Sessions {
			selected := flatIdx == m.cursor
			row := m.renderSessionRow(ss, selected, innerW)
			rows = append(rows, pad+row)
			flatIdx++
		}
	}

	if len(m.flatList) == 0 {
		rows = append(rows, "")
		rows = append(rows, pad+styles.NoSessions.Render("no sessions"))
	}

	// Detail panel for selected session
	var detailRows []string
	if m.cursor < len(m.flatList) {
		ss := m.flatList[m.cursor]
		sep := pad + styles.HelpBar.Render(strings.Repeat("\u2500", min(innerW-4, 50)))
		detailRows = append(detailRows, "", sep)

		if ss.Note != "" {
			detailRows = append(detailRows, pad+styles.HelpBar.Render("note    ")+
				lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Italic(true).Render(ss.Note))
		}
		if ss.PaneInfo != "" {
			detailRows = append(detailRows, pad+styles.HelpBar.Render("panes   ")+styles.SessionName.Render(ss.PaneInfo))
		}

		// Contextual hint based on state
		hint := stateHint(ss)
		if hint != "" {
			detailRows = append(detailRows, pad+styles.HelpBar.Render("hint    ")+styles.HelpBar.Render(hint))
		}
	}

	// Help bar
	helpRow := pad + styles.HelpBar.Render("\u2191\u2193 navigate  enter switch  r refresh  esc back")

	// Assemble
	lines := []string{header}
	lines = append(lines, rows...)
	lines = append(lines, detailRows...)

	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	lines = append(lines, helpRow)

	return strings.Join(lines, "\n")
}

func (m Model) renderSessionRow(ss statusSession, selected bool, maxW int) string {
	cursor := "  "
	if selected {
		cursor = styles.GridSessionSelected.Render("\u25b8 ")
	}

	// Agent indicator
	agent := agentIndicator(ss)

	// repo/name
	label := ss.Session.Repo + "/" + stripRepoPrefix(ss.Session.Name, ss.Session.Repo)
	if selected {
		label = styles.GridSessionSelected.Render(label)
	} else {
		label = styles.GridSessionItem.Render(label)
	}

	// Branch
	branch := ""
	if ss.Branch != "" {
		branch = styles.SessionName.Render("\u2443 " + ss.Branch)
	}

	// Time
	t := ss.Heartbeat
	if t.IsZero() {
		t = ss.Session.LastActiveAt
	}
	if t.IsZero() {
		t = ss.Session.CreatedAt
	}
	age := styles.HelpBar.Render(relativeTime(t))

	// Diff
	diff := renderDiffCompact(ss.Diff)

	parts := []string{cursor + agent + " " + label}
	if branch != "" {
		parts = append(parts, branch)
	}
	if age != "" {
		parts = append(parts, age)
	}
	if diff != "" {
		parts = append(parts, diff)
	}
	return strings.Join(parts, "  ")
}

func agentIndicator(ss statusSession) string {
	if !ss.HasAgent {
		return styles.HelpBar.Render("\u25c7")
	}
	switch ss.State {
	case stateActive:
		return styles.NoteText.Render("\u25c6")
	default:
		return styles.SuccessText.Render("\u25c6")
	}
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

func (m Model) renderSummaryColored() string {
	active, idle, dormant := 0, 0, 0
	for _, sec := range m.sections {
		switch sec.State {
		case stateActive:
			active = len(sec.Sessions)
		case stateIdle:
			idle = len(sec.Sessions)
		case stateDormant:
			dormant = len(sec.Sessions)
		}
	}
	parts := []string{
		styles.NoteText.Render(fmt.Sprintf("%d active", active)),
		styles.HelpBar.Render(fmt.Sprintf("%d idle", idle)),
		styles.HelpBar.Render(fmt.Sprintf("%d dormant", dormant)),
	}
	return strings.Join(parts, styles.HelpBar.Render(" \u00b7 "))
}

func stateHint(ss statusSession) string {
	switch ss.State {
	case stateActive:
		return fmt.Sprintf("agent actively working \u2014 last seen %s ago", relativeTime(ss.Heartbeat))
	case stateIdle:
		return fmt.Sprintf("idle %s \u2014 agent may have finished or is waiting for input", relativeTime(ss.Heartbeat))
	case stateDormant:
		return "no recent activity"
	}
	return ""
}

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
