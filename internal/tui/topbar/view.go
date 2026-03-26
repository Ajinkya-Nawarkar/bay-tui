package topbar

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/constants"
	baytmux "bay/internal/tmux"
	"bay/internal/tui/styles"
)

// View renders the topbar.
func (m Model) View() string {
	w := m.width
	if w < constants.MinTermWidth {
		w = constants.DefaultTermWidth
	}

	// Row 1: "bay" title + repo tabs
	var repoTabs []string
	for i, repo := range m.repos {
		count := m.sessionsForRepo(repo.Name)
		label := repo.Name
		if count > 1 {
			label = fmt.Sprintf("%s (%d)", repo.Name, count)
		}

		if i == m.activeRepoIdx && !m.plusSelected {
			if m.focused && m.focusRow == 0 {
				label = constants.NavRight + label + constants.NavLeft // ▶name (N)◀
				repoTabs = append(repoTabs, styles.RepoTabFocused.Render(label))
			} else {
				repoTabs = append(repoTabs, styles.RepoTabActive.Render(label))
			}
		} else {
			repoTabs = append(repoTabs, styles.RepoTab.Render(label))
		}
	}

	// Append ＋ button
	plusLabel := "＋"
	if m.focused && m.focusRow == 0 && m.plusSelected {
		plusLabel = "\u25b6＋\u25c0" // ▶＋◀
		repoTabs = append(repoTabs, styles.RepoTabFocused.Render(plusLabel))
	} else {
		repoTabs = append(repoTabs, styles.RepoTab.Render(plusLabel))
	}

	titleLabel := "bay focus"
	if m.focused {
		titleLabel = "bay view"
	}
	row1 := styles.Title.Render(titleLabel) + "   " + strings.Join(repoTabs, " \u2502 ")

	if m.mode == modeSettings {
		row1 = styles.Title.Render(titleLabel) + "   " + styles.RepoTabActive.Render("⚙ Settings")
	}
	// Row 2: sessions for active repo (or status line during rename/delete)
	row2 := m.renderSessionRow()

	// Row 3: diff summary
	row3 := m.renderDiffRow()

	// Row 4: session note (or transient status message)
	row4 := m.renderNoteRow()

	// Write hints to file for tmux status bar
	baytmux.WriteTopbarHints(m.renderHintBarPlain())

	content := row1 + "\n" + row2 + "\n" + row3 + "\n" + row4

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
	if m.mode == modeSettings {
		return pad + styles.HelpBar.Render("Editing ~/.bay/config.yaml")
	}
	if m.mode == modeCreate {
		label := "Creating new session..."
		if m.createPreselected != "" {
			label = fmt.Sprintf("Creating session for %s...", m.createPreselected)
		}
		return pad + styles.HelpBar.Render(label)
	}
	if m.mode == modeQuickSwitch {
		return pad + "/ " + m.switchInput.View()
	}
	if m.mode == modeHelp {
		return pad +
			styles.NoteText.Render("←→") + " " + styles.HelpBar.Render("navigate") + "  " +
			styles.NoteText.Render("enter") + " " + styles.HelpBar.Render("activate") + "  " +
			styles.NoteText.Render("n") + " " + styles.HelpBar.Render("new") + "  " +
			styles.NoteText.Render("d") + " " + styles.HelpBar.Render("delete") + "  " +
			styles.NoteText.Render("R") + " " + styles.HelpBar.Render("rename") + "  " +
			styles.NoteText.Render("N") + " " + styles.HelpBar.Render("note") + "  " +
			styles.NoteText.Render("/") + " " + styles.HelpBar.Render("search") + "  " +
			styles.NoteText.Render("m") + " " + styles.HelpBar.Render("memory") + "  " +
			styles.NoteText.Render("S") + " " + styles.HelpBar.Render("settings") + "  " +
			styles.NoteText.Render("q") + " " + styles.HelpBar.Render("quit") + "  " +
			styles.NoteText.Render("esc") + " " + styles.HelpBar.Render("exit")
	}
	if m.mode == modeCleanup {
		var items []string
		start := 0
		if m.cleanupCursor > constants.CleanupPageSize-1 {
			start = m.cleanupCursor - (constants.CleanupPageSize - 1)
		}
		end := start + constants.CleanupPageSize
		if end > len(m.cleanupSessions) {
			end = len(m.cleanupSessions)
		}
		for i := start; i < end; i++ {
			s := m.cleanupSessions[i]
			check := "[ ]"
			if m.cleanupChecked[i] {
				check = "[x]"
			}
			t := s.LastActiveAt
			if t.IsZero() {
				t = s.CreatedAt
			}
			days := int(time.Since(t).Hours() / 24)
			label := fmt.Sprintf("%s %s/%s (%dd ago)", check, s.Repo, s.Name, days)
			if i == m.cleanupCursor {
				items = append(items, styles.SessionTabFocused.Render(label))
			} else {
				items = append(items, styles.SessionTabStale.Render(label))
			}
		}
		return pad + strings.Join(items, "  ")
	}

	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		if m.plusSelected {
			return pad + styles.NoSessions.Render("press enter to create a session")
		}
		return pad + styles.NoSessions.Render("no sessions")
	}

	var tabs []string
	for i, s := range sessions {
		isActive := s.Name == m.activeSession
		isSelected := m.focused && m.focusRow == 1 && i == m.selectedSessionIdx
		stale := isSessionStale(s)

		displayIdx := i + 1
		var label string
		staleMark := ""
		if stale {
			staleMark = " \u2717" // ✗
		}
		if isSelected {
			label = fmt.Sprintf("\u25b6%d:%s%s\u25c0", displayIdx, s.Name, staleMark) // ▶1:name ✗◀
		} else if isActive {
			label = fmt.Sprintf("[%d:%s%s*]", displayIdx, s.Name, staleMark)
		} else {
			label = fmt.Sprintf("[%d:%s%s]", displayIdx, s.Name, staleMark)
		}

		switch {
		case isSelected:
			tabs = append(tabs, styles.SessionTabFocused.Render(label))
		case stale:
			tabs = append(tabs, styles.SessionTabStale.Render(label))
		case isActive:
			tabs = append(tabs, styles.SessionTabActive.Render(label))
		default:
			tabs = append(tabs, styles.SessionTab.Render(label))
		}
	}

	return pad + strings.Join(tabs, " ")
}

func (m Model) renderDiffRow() string {
	pad := "      "

	// Hide during modal modes
	switch m.mode {
	case modeRename, modeConfirmDelete, modeEditNote, modeSettings, modeCreate, modeQuickSwitch, modeHelp, modeCleanup:
		return pad
	}

	sessionName := m.selectedSessionName()
	if sessionName == "" {
		return pad
	}

	// Don't show when browsing repos row
	if m.focused && m.focusRow == 0 {
		return pad
	}

	cached := m.diffCache[sessionName]
	if cached == nil {
		return pad + styles.HelpBar.Render("...")
	}
	if cached.Clean {
		return pad + styles.SuccessText.Render("✓ clean")
	}

	filePart := styles.NoteText.Render(fmt.Sprintf("±%d files", cached.Files))
	insPart := styles.SuccessText.Render(fmt.Sprintf("+%d", cached.Insertions))
	delPart := styles.ErrorText.Render(fmt.Sprintf("-%d", cached.Deletions))
	row := pad + filePart + "  " + insPart + " " + delPart
	if cached.Untracked > 0 {
		row += "  " + styles.HelpKey.Render(fmt.Sprintf("?%d untracked", cached.Untracked))
	}
	if cached.Deleted > 0 {
		row += "  " + styles.ErrorText.Render(fmt.Sprintf("✗%d deleted", cached.Deleted))
	}
	return row
}

func (m Model) renderNoteRow() string {
	pad := "      "
	if m.mode == modeEditNote {
		return pad + "Note: " + m.noteInput.View()
	}
	if m.mode == modeSettings {
		return pad + styles.HelpBar.Render("Save & close editor to return")
	}
	if m.mode == modeCreate {
		return pad + styles.HelpBar.Render("Close wizard to return")
	}
	if m.mode == modeQuickSwitch {
		if len(m.switchMatches) == 0 {
			return pad + styles.NoSessions.Render("no matches")
		}
		var items []string
		limit := 5
		if len(m.switchMatches) < limit {
			limit = len(m.switchMatches)
		}
		for i := 0; i < limit; i++ {
			s := m.switchMatches[i]
			label := s.Repo + "/" + s.Name
			if i == m.switchSelected {
				items = append(items, styles.SessionTabFocused.Render(label))
			} else {
				items = append(items, styles.SessionTab.Render(label))
			}
		}
		return pad + strings.Join(items, "  ")
	}
	if m.mode == modeHelp {
		return pad +
			styles.NoteText.Render("`+space") + " " + styles.HelpBar.Render("focus") + "  " +
			styles.NoteText.Render("`+tab") + " " + styles.HelpBar.Render("cycle") + "  " +
			styles.NoteText.Render("`+1-9") + " " + styles.HelpBar.Render("jump") + "  " +
			styles.NoteText.Render("`+a") + " " + styles.HelpBar.Render("agent") + "  " +
			styles.NoteText.Render("`+d/D") + " " + styles.HelpBar.Render("split") + "  " +
			styles.NoteText.Render("`+w") + " " + styles.HelpBar.Render("close") + "  " +
			styles.NoteText.Render("`+arrows") + " " + styles.HelpBar.Render("nav")
	}
	if m.mode == modeCleanup {
		checked := 0
		for _, c := range m.cleanupChecked {
			if c {
				checked++
			}
		}
		return pad + styles.HelpBar.Render(fmt.Sprintf("%d of %d selected — enter delete, space toggle, a all, esc skip", checked, len(m.cleanupSessions)))
	}
	// Transient status messages take priority over note display
	if m.statusMsg != "" {
		return pad + styles.NoSessions.Render(m.statusMsg)
	}
	note := m.displayedSessionNote()
	if note == "" {
		if m.focused && m.focusRow == 1 {
			return pad + styles.NoSessions.Render("no note — N to add")
		}
		if !m.focused {
			return pad + styles.NoSessions.Render("no note")
		}
		return pad
	}
	return pad + styles.NoteText.Render(note)
}

func (m Model) renderHintBarPlain() string {
	gap := "   "

	if m.mode == modeConfirmDelete {
		return tmuxHint("y", "confirm") + gap + tmuxHint("n", "cancel")
	}
	if m.mode == modeRename {
		return tmuxHint("enter", "save") + gap + tmuxHint("esc", "cancel")
	}

	if m.mode == modeEditNote {
		return tmuxHint("enter", "save") + gap + tmuxHint("esc", "cancel")
	}
	if m.mode == modeSettings {
		return tmuxHint("editing", "close editor to return")
	}
	if m.mode == modeCreate {
		return tmuxHint("creating", "close wizard to return")
	}
	if m.mode == modeQuickSwitch {
		return tmuxHint("↑↓", "select") + gap + tmuxHint("enter", "switch") + gap + tmuxHint("esc", "cancel")
	}
	if m.mode == modeHelp {
		return tmuxHint("any key", "close")
	}
	if m.mode == modeCleanup {
		return tmuxHint("space", "toggle") + gap + tmuxHint("a", "all") + gap + tmuxHint("enter", "delete") + gap + tmuxHint("esc", "skip")
	}

	if m.focused {
		return tmuxHint("←→", "navigate") + gap +
			tmuxHint("enter", "activate") + gap +
			tmuxHint("n", "new") + gap +
			tmuxHint("d", "delete") + gap +
			tmuxHint("R", "rename") + gap +
			tmuxHint("N", "note") + gap +
			tmuxHint("/", "search") + gap +
			tmuxHint("m", "memory") + gap +
			tmuxHint("S", "settings") + gap +
			tmuxHint("?", "help") + gap +
			tmuxHint("q", "quit") + gap +
			tmuxHint("esc", "exit")
	}

	// Unfocused — show prefix shortcuts (navigation → sessions → panes)
	return tmuxHint("`+space", "focus") + gap +
		tmuxHint("`+tab", "cycle") + gap +
		tmuxHint("`+1-9", "jump") + gap +
		tmuxHint("`+a", "agent") + gap +
		tmuxHint("`+d/D", "split") + gap +
		tmuxHint("`+w", "close") + gap +
		tmuxHint("`+arrows", "nav") + gap +
		tmuxHint("`+{/}", "swap")
}

// tmuxHint formats a key+description pair with tmux color codes.
func tmuxHint(key, desc string) string {
	return "#[fg=#FBBF24,bold]" + key + " #[fg=#9CA3AF,nobold]" + desc
}
