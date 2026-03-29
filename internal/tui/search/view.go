package search

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/tui/styles"
)

// View renders the combined search + status screen.
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

	// Header — title left, summary right (when in grouped mode)
	headerLeft := pad + styles.Title.Render("sessions")
	header := headerLeft
	if !m.IsSearching() && m.summary != "" {
		summaryRight := m.renderSummaryColored()
		gap := innerW - lipgloss.Width(headerLeft) - lipgloss.Width(summaryRight)
		if gap < 2 {
			gap = 2
		}
		header = headerLeft + strings.Repeat(" ", gap) + summaryRight
	}

	// Input
	inputRow := pad + "\U0001F50D " + m.input.View()

	// Body — either grouped sections or flat filtered list
	maxRows := h - 10
	if maxRows < 3 {
		maxRows = 3
	}

	var bodyRows []string
	if m.IsSearching() {
		bodyRows = m.renderFilteredList(pad, innerW, maxRows)
	} else {
		bodyRows = m.renderGroupedSections(pad, innerW, maxRows)
	}

	// Detail panel
	detailRows := m.renderDetailPanel(pad, innerW)

	// Help bar
	helpRow := pad + styles.HelpBar.Render(m.helpText())

	// Assemble
	lines := []string{header, "", inputRow, ""}
	lines = append(lines, bodyRows...)
	lines = append(lines, "")
	lines = append(lines, detailRows...)

	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	lines = append(lines, helpRow)

	return strings.Join(lines, "\n")
}

func (m Model) renderFilteredList(pad string, innerW, maxRows int) []string {
	var rows []string
	if len(m.filtered) == 0 {
		rows = append(rows, pad+styles.NoSessions.Render("no matches"))
		return rows
	}

	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := start; i < end; i++ {
		row := m.renderSessionRow(m.filtered[i], i == m.cursor, innerW)
		rows = append(rows, pad+row)
	}

	if end < len(m.filtered) {
		rows = append(rows, pad+styles.HelpBar.Render(fmt.Sprintf("  ... %d more below", len(m.filtered)-end)))
	}
	return rows
}

func (m Model) renderGroupedSections(pad string, innerW, maxRows int) []string {
	if len(m.sections) == 0 {
		return []string{pad + styles.NoSessions.Render("no sessions")}
	}

	var rows []string
	flatIdx := 0
	for _, sec := range m.sections {
		rows = append(rows, pad+styles.Subtitle.Render(sec.Label))
		for _, e := range sec.Sessions {
			row := m.renderSessionRow(e, flatIdx == m.cursor, innerW)
			rows = append(rows, pad+row)
			flatIdx++
		}
		rows = append(rows, "") // gap between sections
	}
	return rows
}

func (m Model) renderDetailPanel(pad string, innerW int) []string {
	list := m.visibleList()
	if m.cursor >= len(list) {
		return nil
	}
	e := list[m.cursor]

	var rows []string
	sep := pad + styles.HelpBar.Render(strings.Repeat("\u2500", min(innerW-4, 50)))
	rows = append(rows, sep)

	if e.Note != "" {
		rows = append(rows, pad+styles.HelpBar.Render("note    ")+
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Italic(true).Render(e.Note))
	}
	if e.PaneInfo != "" {
		rows = append(rows, pad+styles.HelpBar.Render("panes   ")+styles.SessionName.Render(e.PaneInfo))
	}

	// State hint (only in grouped mode)
	if !m.IsSearching() {
		hint := stateHint(e)
		if hint != "" {
			rows = append(rows, pad+styles.HelpBar.Render("hint    ")+styles.HelpBar.Render(hint))
		}
	}

	return rows
}

func (m Model) helpText() string {
	list := m.visibleList()
	if len(list) == 0 {
		return "esc back"
	}
	if m.IsSearching() && len(list) == 1 {
		return fmt.Sprintf("enter to switch to %s  esc back", list[0].Session.Name)
	}
	base := fmt.Sprintf("%d sessions  \u2191\u2193 navigate  enter switch  ctrl+r refresh  esc back", len(list))
	return base
}

func (m Model) renderSessionRow(e enrichedSession, selected bool, maxW int) string {
	cursor := "  "
	if selected {
		cursor = styles.GridSessionSelected.Render("\u25b8 ")
	}

	// Agent indicator
	agent := agentIndicator(e)

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

	// Time
	t := e.Heartbeat
	if t.IsZero() {
		t = e.Session.LastActiveAt
	}
	if t.IsZero() {
		t = e.Session.CreatedAt
	}
	age := styles.HelpBar.Render(relativeTime(t))

	// Diff
	diff := renderDiffCompact(e.Diff)

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

func agentIndicator(e enrichedSession) string {
	if !e.HasAgent {
		return styles.HelpBar.Render("\u25c7")
	}
	if e.AgentActive {
		return styles.NoteText.Render("\u25c6")
	}
	return styles.SuccessText.Render("\u25c6")
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
		switch sec.Label {
		case "ACTIVE":
			active = len(sec.Sessions)
		case "IDLE":
			idle = len(sec.Sessions)
		case "DORMANT":
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

func stateHint(e enrichedSession) string {
	switch e.State {
	case stateActive:
		return fmt.Sprintf("agent actively working \u2014 last seen %s ago", relativeTime(e.Heartbeat))
	case stateIdle:
		return fmt.Sprintf("idle %s \u2014 agent may have finished or is waiting for input", relativeTime(e.Heartbeat))
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
