package purpose

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/tui/styles"
)

// View renders the purpose view.
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

	// Header
	sessionName := ""
	if m.session != nil {
		sessionName = m.session.Name
	}
	headerLeft := pad + styles.Title.Render("session purpose")
	headerRight := styles.HelpBar.Render(sessionName)
	gap := innerW - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if gap < 2 {
		gap = 2
	}
	header := headerLeft + strings.Repeat(" ", gap) + headerRight

	// Purpose section
	var purposeRows []string
	if m.mode == modeEditPurpose {
		purposeRows = append(purposeRows, pad+styles.Subtitle.Render("Purpose:")+" "+styles.HelpBar.Render("[editing]"))
		purposeRows = append(purposeRows, pad+m.input.View())
	} else {
		purposeRows = append(purposeRows, pad+styles.Subtitle.Render("Purpose:"))
		if m.purpose != "" {
			purposeRows = append(purposeRows, pad+lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Render(m.purpose))
		} else {
			purposeRows = append(purposeRows, pad+styles.NoSessions.Render("not set — press e to define"))
		}
	}

	// Checklist section
	var checklistRows []string
	if m.mode == modeAddItem {
		checklistRows = append(checklistRows, pad+styles.Subtitle.Render("Checklist:")+" "+styles.HelpBar.Render("[adding]"))
	} else {
		checklistRows = append(checklistRows, pad+styles.Subtitle.Render("Checklist:"))
	}

	if len(m.items) == 0 && m.mode != modeAddItem {
		checklistRows = append(checklistRows, pad+styles.NoSessions.Render("no items — press a to add"))
	} else {
		for i, item := range m.items {
			marker := "[ ]"
			markerStyle := styles.HelpBar
			if item.Status == "done" {
				marker = "[x]"
				markerStyle = styles.SuccessText
			}

			cursor := "  "
			if i == m.cursor && m.mode == modeBrowse {
				cursor = styles.GridSessionSelected.Render("\u25b8 ")
			}

			title := item.Title
			if item.Status == "done" {
				title = styles.HelpBar.Render(title)
			} else {
				title = styles.SessionName.Render(title)
			}

			checklistRows = append(checklistRows, pad+cursor+markerStyle.Render(marker)+" "+title)
		}
	}

	if m.mode == modeAddItem {
		checklistRows = append(checklistRows, pad+"  "+m.input.View())
	}

	// Help bar
	var helpText string
	switch m.mode {
	case modeBrowse:
		helpText = "e edit purpose  a add item  d toggle done  x remove  esc back"
	case modeEditPurpose:
		helpText = "enter save  esc cancel"
	case modeAddItem:
		helpText = "enter add  esc cancel"
	}
	helpRow := pad + styles.HelpBar.Render(helpText)

	// Assemble
	lines := []string{header, ""}
	lines = append(lines, purposeRows...)
	lines = append(lines, "")
	lines = append(lines, checklistRows...)

	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	lines = append(lines, helpRow)

	_ = fmt.Sprintf // suppress unused import
	return strings.Join(lines, "\n")
}
