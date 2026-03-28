package archive

import (
	"fmt"
	"strings"
	"time"

	"bay/internal/constants"
	"bay/internal/tui/styles"
)

// View renders the archive browser.
func (m Model) View() string {
	w := m.width
	if w < constants.MinTermWidth {
		w = constants.DefaultTermWidth
	}

	var b strings.Builder

	// Search bar
	if m.searching {
		b.WriteString("\n  " + styles.Prompt.Render("/ ") + m.search.View() + "\n")
	} else if m.search.Value() != "" {
		b.WriteString("\n  " + styles.HelpBar.Render("/ "+m.search.Value()) +
			"  " + styles.Subtitle.Render(fmt.Sprintf("(%d matches)", len(m.filtered))) + "\n")
	}

	b.WriteString("\n")

	// Session list
	if len(m.filtered) == 0 {
		if len(m.sessions) == 0 {
			b.WriteString("  " + styles.NoSessions.Render("No archived sessions") + "\n")
		} else {
			b.WriteString("  " + styles.NoSessions.Render("No matches") + "\n")
		}
	} else {
		start, end := m.visibleRange()
		for i := start; i < end; i++ {
			s := m.filtered[i]
			days := int(time.Since(s.ArchivedAt).Hours() / 24)
			age := fmt.Sprintf("archived %dd ago", days)
			if days == 0 {
				age = "archived today"
			} else if days == 1 {
				age = "archived 1d ago"
			}
			label := fmt.Sprintf("%-35s %s", s.Repo+"/"+s.Name, age)

			if i == m.cursor {
				b.WriteString("  " + styles.GridSessionSelected.Render("▸ "+label) + "\n")
			} else {
				b.WriteString("  " + styles.GridSessionStale.Render("  "+label) + "\n")
			}
		}

		// Scroll indicator
		if len(m.filtered) > constants.ArchivePageSize {
			b.WriteString(fmt.Sprintf("\n  "+styles.HelpBar.Render("%d of %d"), m.cursor+1, len(m.filtered)))
			b.WriteString("\n")
		}
	}

	// Help bar
	b.WriteString("\n")
	if m.searching {
		b.WriteString("  " + styles.HelpBar.Render("enter confirm  esc clear"))
	} else {
		b.WriteString("  " +
			styles.NoteText.Render("u") + " " + styles.HelpBar.Render("unarchive") + "  " +
			styles.NoteText.Render("d") + " " + styles.HelpBar.Render("delete") + "  " +
			styles.NoteText.Render("U") + " " + styles.HelpBar.Render("unarchive all") + "  " +
			styles.NoteText.Render("D") + " " + styles.HelpBar.Render("delete all") + "  " +
			styles.NoteText.Render("/") + " " + styles.HelpBar.Render("search") + "  " +
			styles.NoteText.Render("esc") + " " + styles.HelpBar.Render("back"))
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n\n  " + styles.SuccessText.Render(m.statusMsg))
	}

	b.WriteString("\n")
	return b.String()
}

// visibleRange returns the start/end indices for the visible window.
func (m Model) visibleRange() (int, int) {
	total := len(m.filtered)
	if total <= constants.ArchivePageSize {
		return 0, total
	}
	start := 0
	if m.cursor > constants.ArchivePageSize-1 {
		start = m.cursor - (constants.ArchivePageSize - 1)
	}
	end := start + constants.ArchivePageSize
	if end > total {
		end = total
	}
	return start, end
}
