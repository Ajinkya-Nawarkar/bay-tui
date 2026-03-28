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

// View renders the topbar in either collapsed (unfocused) or expanded (focused) mode.
func (m Model) View() string {
	w := m.width
	if w < constants.MinTermWidth {
		w = constants.DefaultTermWidth
	}

	// Write hints to file for tmux status bar
	baytmux.WriteTopbarHints(m.renderHintBarPlain())

	var content string
	if !m.focused && m.mode == modeNormal {
		content = m.renderCollapsedView(w)
	} else {
		content = m.renderExpandedView(w)
	}

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

// renderCollapsedView renders the 2-row unfocused topbar.
// Row 1: bay · {active_repo}                      ±N +N -N
// Row 2: session1*  session2  session3  ...
func (m Model) renderCollapsedView(w int) string {
	pad := "  "

	// Row 1: title + active repo
	row1 := pad + styles.CollapsedTitle.Render("bay")
	if m.activeSession != "" {
		for _, s := range m.sessions {
			if s.Name == m.activeSession {
				row1 += " " + styles.CollapsedRepo.Render("· "+s.Repo)
				break
			}
		}
	}

	// Row 2: recent sessions (MRU from hot row, fill to width)
	// Same-repo sessions get a distinct style to stand out from cross-repo ones.
	activeRepo := ""
	for _, s := range m.sessions {
		if s.Name == m.activeSession {
			activeRepo = s.Repo
			break
		}
	}

	superscripts := []string{"¹", "²", "³", "⁴", "⁵", "⁶", "⁷", "⁸", "⁹"}

	var items []string
	availWidth := w - len(pad) - 4 // border + padding
	usedWidth := 0
	for idx, s := range m.hotRow {
		label := stripRepoPrefix(s.Name, s.Repo)
		sameRepo := s.Repo == activeRepo

		num := ""
		if idx < len(superscripts) {
			num = styles.HelpBar.Render(superscripts[idx])
		}

		dot := m.diffDot(s.Name)
		age := staleTime(s.LastActiveAt)
		pulse := m.agentPulse(s.Name)

		var rendered string
		if s.Name == m.activeSession {
			rendered = num + styles.CollapsedSessionSameRepo.Render("[") +
				dot + styles.CollapsedSessionActive.Render(label) + pulse +
				styles.CollapsedSessionSameRepo.Render("]")
		} else if sameRepo {
			rendered = num + styles.CollapsedSessionSameRepo.Render("[") +
				dot + styles.CollapsedSession.Render(label) + pulse +
				styles.CollapsedSessionSameRepo.Render("]") + age
		} else {
			rendered = num + styles.CollapsedSession.Render("[") +
				dot + styles.CollapsedSession.Render(label) + pulse +
				styles.CollapsedSession.Render("]") + age
		}

		labelW := lipgloss.Width(rendered) + 2 // +2 for spacing
		if usedWidth+labelW > availWidth && len(items) > 0 {
			break
		}
		items = append(items, rendered)
		usedWidth += labelW
	}

	row2 := pad
	if len(items) == 0 {
		row2 += styles.NoSessions.Render("no sessions")
	} else {
		row2 += strings.Join(items, "  ")
	}

	// Row 3: note (left) + branch + diff (right)
	var noteLeft, branchDiffRight string
	if m.activeSession != "" {
		for _, s := range m.sessions {
			if s.Name == m.activeSession {
				if s.Note != "" {
					noteLeft = styles.CollapsedNote.Render(s.Note)
				}
				if s.WorktreeBranch != "" {
					branchDiffRight = styles.SessionName.Render("⑃ "+s.WorktreeBranch)
				}
				break
			}
		}
	}
	diff := m.renderDiffInline()
	if diff != "" {
		if branchDiffRight != "" {
			branchDiffRight += "  " + diff
		} else {
			branchDiffRight = diff
		}
	}

	row3 := pad
	if noteLeft != "" || branchDiffRight != "" {
		rightW := lipgloss.Width(branchDiffRight)
		leftW := lipgloss.Width(pad + noteLeft)
		gap := w - leftW - rightW - 4 // border + padding
		if gap < 1 {
			gap = 1
		}
		row3 = pad + noteLeft + strings.Repeat(" ", gap) + branchDiffRight
	}

	return row1 + "\n" + row2 + "\n" + row3
}

// renderExpandedView renders the horizontal expanded view for focused/modal modes.
// Row 1: header + diff. Row 2: repo tabs. Row 3: sessions for selected repo. Row 4: note.
func (m Model) renderExpandedView(w int) string {
	pad := "  "

	// Handle modal modes — they replace the body
	if m.mode != modeNormal {
		return m.renderModalContent(w, pad)
	}

	// Row 1: header + diff (right-aligned)
	left := styles.CollapsedTitle.Render("bay")
	diff := m.renderDiffInline()
	gap := w - lipgloss.Width(pad+left) - lipgloss.Width(diff) - 4
	if gap < 1 {
		gap = 1
	}
	header := pad + left + strings.Repeat(" ", gap) + diff

	// Row 2: horizontal repo tabs
	var repoTabs []string
	for i, repo := range m.repos {
		isSelected := m.focused && m.focusRow == 0 && i == m.activeRepoIdx && !m.plusSelected
		isActiveRepo := false
		for _, s := range m.sessions {
			if s.Repo == repo.Name && s.Name == m.activeSession {
				isActiveRepo = true
				break
			}
		}

		label := repo.Name
		switch {
		case isSelected:
			label = constants.NavRight + label + constants.NavLeft
			repoTabs = append(repoTabs, styles.GridRepoNameFocused.Render(label))
		case isActiveRepo:
			repoTabs = append(repoTabs, styles.GridRepoNameActive.Render("▸"+label))
		default:
			repoTabs = append(repoTabs, styles.GridRepoName.Render(label))
		}
	}
	// ＋ button on repo row
	if m.focused && m.plusSelected {
		repoTabs = append(repoTabs, styles.GridPlusFocused.Render(constants.NavRight+"＋"+constants.NavLeft))
	} else {
		repoTabs = append(repoTabs, styles.GridPlus.Render("＋"))
	}
	repoRow := pad + strings.Join(repoTabs, "  ")

	// Row 3: sessions for the selected repo
	sessions := m.activeRepoSessions()
	var sessionItems []string
	repoName := ""
	if m.activeRepoIdx < len(m.repos) {
		repoName = m.repos[m.activeRepoIdx].Name
	}
	for j, s := range sessions {
		label := stripRepoPrefix(s.Name, repoName)
		stale := isSessionStale(s)
		isActive := s.Name == m.activeSession
		isSelected := m.focused && m.focusRow == 1 && j == m.selectedSessionIdx
		dot := m.diffDot(s.Name)
		pulse := m.agentPulse(s.Name)

		switch {
		case isSelected:
			label = constants.NavRight + label + pulse + constants.NavLeft
			sessionItems = append(sessionItems, dot+styles.GridSessionSelected.Render(label))
		case stale:
			sessionItems = append(sessionItems, dot+styles.GridSessionStale.Render(label)+pulse)
		case isActive:
			sessionItems = append(sessionItems, dot+styles.GridSessionActive.Render(label)+pulse)
		default:
			sessionItems = append(sessionItems, dot+styles.GridSessionItem.Render(label)+pulse)
		}
	}
	sessionRow := pad
	if len(sessionItems) == 0 {
		sessionRow += styles.NoSessions.Render("no sessions — n to create")
	} else {
		sessionRow += strings.Join(sessionItems, "  ")
	}

	// Row 4: info row — note (left) + branch + diff (right), same as collapsed
	rows := []string{header, repoRow, sessionRow}
	if m.focused && m.focusRow == 1 && !m.plusSelected {
		sessionName := m.selectedSessionName()
		var noteLeft, branchDiffRight string
		for _, s := range m.sessions {
			if s.Name == sessionName {
				if s.Note != "" {
					noteLeft = styles.CollapsedNote.Render(s.Note)
				}
				if s.WorktreeBranch != "" {
					branchDiffRight = styles.SessionName.Render("⑃ " + s.WorktreeBranch)
				}
				break
			}
		}
		diff := m.renderDiffInline()
		if diff != "" {
			if branchDiffRight != "" {
				branchDiffRight += "  " + diff
			} else {
				branchDiffRight = diff
			}
		}
		if noteLeft != "" || branchDiffRight != "" {
			leftW := lipgloss.Width(pad + noteLeft)
			rightW := lipgloss.Width(branchDiffRight)
			gap := w - leftW - rightW - 4
			if gap < 1 {
				gap = 1
			}
			rows = append(rows, pad+noteLeft+strings.Repeat(" ", gap)+branchDiffRight)
		}
	}

	return strings.Join(rows, "\n")
}

// renderModalContent renders modal UI (rename, delete, search, etc.) within the expanded border.
func (m Model) renderModalContent(w int, pad string) string {
	header := pad + styles.CollapsedTitle.Render("bay")

	switch m.mode {
	case modeConfirmDelete:
		return header + "\n" + pad + fmt.Sprintf("Delete '%s'? (y/n)", m.deleteTarget)
	case modeRename:
		return header + "\n" + pad + "Rename: " + m.renameInput.View()
	case modeEditNote:
		return header + "\n" + pad + "Note: " + m.noteInput.View()
	case modeSettings:
		return header + "\n" + pad + styles.HelpBar.Render("Editing ~/.bay/config.yaml") +
			"\n" + pad + styles.HelpBar.Render("Save & close editor to return")
	case modeCreate:
		label := "Creating new session..."
		if m.createPreselected != "" {
			label = fmt.Sprintf("Creating session for %s...", m.createPreselected)
		}
		return header + "\n" + pad + styles.HelpBar.Render(label) +
			"\n" + pad + styles.HelpBar.Render("Close wizard to return")
	case modeGlobalSearch:
		row := pad + "\U0001F50D " + m.switchInput.View()
		// Match list
		if len(m.globalSearchMatches) > 0 {
			var items []string
			limit := 5
			if len(m.globalSearchMatches) < limit {
				limit = len(m.globalSearchMatches)
			}
			for i := 0; i < limit; i++ {
				s := m.globalSearchMatches[i]
				label := s.Repo + "/" + s.Name + "  " + relativeTime(s.LastActiveAt)
				if i == m.globalSearchSelected {
					items = append(items, styles.GridSessionSelected.Render(label))
				} else {
					items = append(items, styles.GridSessionItem.Render(label))
				}
			}
			row += "\n" + pad + strings.Join(items, "  ")
		}
		count := len(m.globalSearchMatches)
		if count == 0 {
			row += "\n" + pad + styles.NoSessions.Render("no matches — tab navigate, esc cancel")
		} else {
			row += "\n" + pad + styles.HelpBar.Render(fmt.Sprintf("%d matches — tab navigate, enter switch, esc cancel", count))
		}
		return header + "\n" + row
	case modeHelp:
		return header + "\n" +
			pad + styles.NoteText.Render("←→") + " " + styles.HelpBar.Render("navigate") + "  " +
			styles.NoteText.Render("enter") + " " + styles.HelpBar.Render("activate") + "  " +
			styles.NoteText.Render("n") + " " + styles.HelpBar.Render("new") + "  " +
			styles.NoteText.Render("d") + " " + styles.HelpBar.Render("delete") + "  " +
			styles.NoteText.Render("R") + " " + styles.HelpBar.Render("rename") + "  " +
			styles.NoteText.Render("N") + " " + styles.HelpBar.Render("note") + "  " +
			styles.NoteText.Render("/") + " " + styles.HelpBar.Render("search") + "  " +
			styles.NoteText.Render("m") + " " + styles.HelpBar.Render("memory") + "  " +
			styles.NoteText.Render("S") + " " + styles.HelpBar.Render("settings") + "  " +
			styles.NoteText.Render("q") + " " + styles.HelpBar.Render("quit") + "  " +
			styles.NoteText.Render("esc") + " " + styles.HelpBar.Render("exit") + "\n" +
			pad + styles.NoteText.Render("`+space") + " " + styles.HelpBar.Render("focus") + "  " +
			styles.NoteText.Render("`+tab") + " " + styles.HelpBar.Render("cycle") + "  " +
			styles.NoteText.Render("`+1-9") + " " + styles.HelpBar.Render("jump") + "  " +
			styles.NoteText.Render("`+a") + " " + styles.HelpBar.Render("agent") + "  " +
			styles.NoteText.Render("`+d/D") + " " + styles.HelpBar.Render("split") + "  " +
			styles.NoteText.Render("`+w") + " " + styles.HelpBar.Render("close") + "  " +
			styles.NoteText.Render("`+arrows") + " " + styles.HelpBar.Render("nav")
	case modeCleanup:
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
				items = append(items, styles.GridSessionSelected.Render(label))
			} else {
				items = append(items, styles.GridSessionStale.Render(label))
			}
		}
		checked := 0
		for _, c := range m.cleanupChecked {
			if c {
				checked++
			}
		}
		return header + "\n" + pad + strings.Join(items, "  ") +
			"\n" + pad + styles.HelpBar.Render(fmt.Sprintf("%d of %d selected — enter delete, space toggle, a all, esc skip", checked, len(m.cleanupSessions)))
	}

	return header
}

// renderDiffInline renders the diff summary as a compact inline string.
func (m Model) renderDiffInline() string {
	sessionName := m.activeSession
	if sessionName == "" {
		return ""
	}

	cached := m.diffCache[sessionName]
	if cached == nil {
		return ""
	}
	if cached.Clean {
		return styles.SuccessText.Render("✓ clean")
	}

	return styles.NoteText.Render(fmt.Sprintf("±%d", cached.Files)) + " " +
		styles.SuccessText.Render(fmt.Sprintf("+%d", cached.Insertions)) + " " +
		styles.ErrorText.Render(fmt.Sprintf("-%d", cached.Deletions))
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
	if m.mode == modeGlobalSearch {
		return tmuxHint("tab/↑↓", "navigate") + gap + tmuxHint("enter", "switch") + gap + tmuxHint("esc", "cancel")
	}
	if m.mode == modeHelp {
		return tmuxHint("any key", "close")
	}
	if m.mode == modeCleanup {
		return tmuxHint("space", "toggle") + gap + tmuxHint("a", "all") + gap + tmuxHint("enter", "delete") + gap + tmuxHint("esc", "skip")
	}

	if m.focused {
		return tmuxHint("←→↑↓", "navigate") + gap +
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

	// Unfocused — show prefix shortcuts
	return tmuxHint("`+space", "focus") + gap +
		tmuxHint("`+tab", "cycle") + gap +
		tmuxHint("`+/", "search") + gap +
		tmuxHint("`+1-9", "jump") + gap +
		tmuxHint("`+a", "agent") + gap +
		tmuxHint("`+d/D", "split") + gap +
		tmuxHint("`+w", "close") + gap +
		tmuxHint("`+arrows", "nav") + gap +
		tmuxHint("`+{/}", "swap")
}

// --- Helpers ---

// relativeTime returns a short human-readable time like "now", "2m", "1h", "3d".
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// tmuxHint formats a key+description pair with tmux color codes.
func tmuxHint(key, desc string) string {
	return "#[fg=#FBBF24,bold]" + key + " #[fg=#9CA3AF,nobold]" + desc
}

// staleTimeThreshold is the minimum age before showing a relative time indicator.
const staleTimeThreshold = 24 * time.Hour

// diffDot returns a colored git symbol indicating git status for a session.
// Green ⑃ = clean, orange ⑃ = dirty, dim ⑃ = no data yet.
func (m Model) diffDot(sessionName string) string {
	cached := m.diffCache[sessionName]
	if cached == nil {
		return styles.HelpBar.Render("⑃ ")
	}
	if cached.Clean {
		return styles.SuccessText.Render("⑃ ")
	}
	return styles.NoteText.Render("⑃ ")
}

// staleTime returns a dim relative time string if the session is older than the threshold.
func staleTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < staleTimeThreshold {
		return ""
	}
	var label string
	switch {
	case d < 48*time.Hour:
		label = "1d"
	case d < 7*24*time.Hour:
		label = fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		label = fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	}
	return " " + styles.HelpBar.Render(label)
}

// agentPulse returns a diamond indicator for agent status.
// ◇ hollow (dim) = no agent panes
// ◆ green = agent panes exist, all idle
// ◆ red = agent panes exist, at least one actively working
func (m Model) agentPulse(sessionName string) string {
	// Check YAML pane data OR heartbeat file — either means agents exist
	hasAgent := false
	for _, s := range m.sessions {
		if s.Name == sessionName {
			for _, p := range s.Panes {
				if p.Type == "agent" {
					hasAgent = true
					break
				}
			}
			break
		}
	}
	// Heartbeat file existence also indicates an agent (YAML may lag behind)
	if !hasAgent {
		if _, ok := m.agentActive[sessionName]; ok {
			hasAgent = true
		}
	}
	if !hasAgent {
		return styles.HelpBar.Render(" ◇")
	}
	if m.isAgentActive(sessionName) {
		return styles.NoteText.Render(" ◆")
	}
	return styles.SuccessText.Render(" ◆")
}

// stripRepoPrefix removes the "repo-" prefix from a session name for cleaner display.
func stripRepoPrefix(name, repo string) string {
	prefix := strings.ToLower(repo) + "-"
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, prefix) && len(name) > len(prefix) {
		return name[len(prefix):]
	}
	return name
}

// truncate shortens a string to maxLen with "…" if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

// padRight pads a string with spaces to the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
