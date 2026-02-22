package sidebar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/anawarkar/bay/internal/tui/styles"
)

// View renders the sidebar.
func (m Model) View() string {
	w := m.width
	if w < 10 {
		w = 35
	}
	innerW := w - 4 // account for box borders + padding

	// Title bar
	title := styles.Title.Render(" bay")

	// Tree section in a box
	treeContent := m.tree.View()
	treeBox := styles.SectionBox.
		Width(innerW).
		Render(treeContent)

	// Status line
	statusLine := ""
	if s := m.viewStatusLine(); s != "" {
		statusLine = styles.StatusBar.Render(truncate(s, innerW))
	}

	// Help keys - render in a compact grid
	helpBox := styles.SectionBox.
		Width(innerW).
		BorderForeground(styles.Dim).
		Render(renderHelp(innerW))

	// Assemble
	var sections []string
	sections = append(sections, title)
	sections = append(sections, treeBox)
	if statusLine != "" {
		sections = append(sections, statusLine)
	}
	sections = append(sections, helpBox)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Constrain to terminal height and pad every line to full width.
	// Each line must fill the full width to overwrite previous frame content.
	lines := strings.Split(content, "\n")
	if m.height > 0 {
		if len(lines) > m.height {
			lines = lines[:m.height]
		}
		for len(lines) < m.height {
			lines = append(lines, "")
		}
	}

	// Pad each line to full width with spaces to clear ghost artifacts
	if w > 0 {
		blank := strings.Repeat(" ", w)
		for i, line := range lines {
			// Use visual length — ANSI escapes don't count
			visible := lipgloss.Width(line)
			if visible < w {
				lines[i] = line + blank[:w-visible]
			}
		}
	}

	return strings.Join(lines, "\n")
}

func renderHelp(width int) string {
	keys := []struct{ key, label string }{
		{"n", "new"},
		{"d", "del"},
		{"r", "ren"},
		{"c", "claude"},
		{"s", "setup"},
		{"q", "quit"},
	}

	var parts []string
	for _, k := range keys {
		part := fmt.Sprintf("%s %s",
			styles.HelpKey.Render(k.key),
			styles.HelpBar.Render(k.label),
		)
		parts = append(parts, part)
	}

	// Arrange in rows that fit the width
	row1 := strings.Join(parts[:3], "  ")
	row2 := strings.Join(parts[3:], "  ")
	return row1 + "\n" + row2
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
