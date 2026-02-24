package topbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/tui/styles"
)

// View renders the topbar.
func (m Model) View() string {
	w := m.width
	if w < 20 {
		w = 80
	}

	// Row 1: "bay" title + repo tabs
	var repoTabs []string
	for i, repo := range m.repos {
		if i == m.activeRepoIdx {
			name := repo.Name
			if m.focused && m.focusRow == 0 {
				name = "\u25b6" + name + "\u25c0" // ▶name◀
				repoTabs = append(repoTabs, styles.RepoTabFocused.Render(name))
			} else {
				repoTabs = append(repoTabs, styles.RepoTabActive.Render(name))
			}
		} else {
			repoTabs = append(repoTabs, styles.RepoTab.Render(repo.Name))
		}
	}

	row1 := styles.Title.Render("bay") + "   " + strings.Join(repoTabs, " \u2502 ")

	// Row 2: sessions for active repo (or status line during rename/delete)
	row2 := m.renderSessionRow()

	// Status message overlays row2 when set
	if m.statusMsg != "" {
		row2 = "      " + styles.NoSessions.Render(m.statusMsg)
	}

	content := row1 + "\n" + row2

	// Choose border color based on focus state
	var box lipgloss.Style
	if m.focused {
		box = styles.FocusedBorder.Width(w - 2)
	} else {
		box = styles.SectionBox.Width(w - 2)
	}

	result := box.Render(content)

	// Pad to full width and constrain to terminal height
	lines := strings.Split(result, "\n")
	if w > 0 {
		blank := strings.Repeat(" ", w)
		for i, line := range lines {
			visible := lipgloss.Width(line)
			if visible < w {
				lines[i] = line + blank[:w-visible]
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderSessionRow() string {
	pad := "      "

	if m.mode == modeConfirmDelete {
		return pad + fmt.Sprintf("Delete '%s'? (y/n)", m.deleteTarget)
	}
	if m.mode == modeRename {
		return pad + "Rename: " + m.renameInput.View()
	}

	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		return pad + styles.NoSessions.Render("no sessions")
	}

	var tabs []string
	for i, s := range sessions {
		isActive := s.Name == m.activeSession
		isSelected := m.focused && m.focusRow == 1 && i == m.selectedSessionIdx

		var label string
		if isSelected {
			label = fmt.Sprintf("\u25b6%d:%s\u25c0", i, s.Name) // ▶0:name◀
		} else if isActive {
			label = fmt.Sprintf("[%d:%s*]", i, s.Name)
		} else {
			label = fmt.Sprintf("[%d:%s]", i, s.Name)
		}

		switch {
		case isSelected:
			tabs = append(tabs, styles.SessionTabFocused.Render(label))
		case isActive:
			tabs = append(tabs, styles.SessionTabActive.Render(label))
		default:
			tabs = append(tabs, styles.SessionTab.Render(label))
		}
	}

	return pad + strings.Join(tabs, " ")
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
