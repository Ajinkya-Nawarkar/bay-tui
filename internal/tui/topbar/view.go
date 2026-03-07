package topbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	baytmux "bay/internal/tmux"
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

	// Write hints to file for tmux status bar
	baytmux.WriteTopbarHints(m.renderHintBarPlain())

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

		// Display 1-indexed: session 0 shows as 1, ..., session 9 shows as 0
		displayIdx := (i + 1) % 10
		var label string
		if isSelected {
			label = fmt.Sprintf("\u25b6%d:%s\u25c0", displayIdx, s.Name) // ▶1:name◀
		} else if isActive {
			label = fmt.Sprintf("[%d:%s*]", displayIdx, s.Name)
		} else {
			label = fmt.Sprintf("[%d:%s]", displayIdx, s.Name)
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

func (m Model) renderHintBarPlain() string {
	gap := "   "

	if m.mode == modeConfirmDelete {
		return tmuxHint("y", "confirm") + gap + tmuxHint("n", "cancel")
	}
	if m.mode == modeRename {
		return tmuxHint("enter", "save") + gap + tmuxHint("esc", "cancel")
	}

	if m.focused {
		return tmuxHint("←→", "navigate") + gap +
			tmuxHint("enter", "activate") + gap +
			tmuxHint("n", "new") + gap +
			tmuxHint("d", "delete") + gap +
			tmuxHint("R", "rename") + gap +
			tmuxHint("m", "memory") + gap +
			tmuxHint("q", "quit") + gap +
			tmuxHint("esc", "exit")
	}

	// Unfocused — show prefix shortcuts (navigation → sessions → panes)
	return tmuxHint("`+space", "focus") + gap +
		tmuxHint("`+tab", "cycle") + gap +
		tmuxHint("`+1-0", "jump") + gap +
		tmuxHint("`+r", "repo") + gap +
		tmuxHint("`+a", "agent") + gap +
		tmuxHint("`+d/D", "split") + gap +
		tmuxHint("`+w", "close") + gap +
		tmuxHint("`+s", "toggle") + gap +
		tmuxHint("`+arrows", "nav")
}

// tmuxHint formats a key+description pair with tmux color codes.
func tmuxHint(key, desc string) string {
	return "#[fg=#F97316,bold]" + key + " #[fg=#9CA3AF,nobold]" + desc
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
