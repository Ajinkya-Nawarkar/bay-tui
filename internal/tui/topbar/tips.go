package topbar

import (
	"math/rand"
	"time"

	"github.com/charmbracelet/lipgloss"

	"bay/internal/constants"
	"bay/internal/session"
	"bay/internal/tui/styles"
)

// tip is a single usage hint shown in the topbar.
type tip struct {
	Key  string
	Desc string
}

// Base tips for repos row — archive tip added dynamically.
var tipsFocusedReposBase = []tip{
	{"← →", "switch repos"},
	{"↓", "browse sessions"},
	{"n", "new session"},
	{"/", "search all"},
	{"s", "session status"},
}

// currentTips returns tips relevant to the current topbar state.
// Built dynamically based on session count, notes, etc.
func (m Model) currentTips() []tip {
	if m.mode != modeNormal {
		return nil
	}

	if !m.focused {
		return m.unfocusedTips()
	}
	if m.focusRow == 0 {
		return m.focusedRepoTips()
	}
	return m.focusedSessionTips()
}

// unfocusedTips builds tips for collapsed mode based on session state.
func (m Model) unfocusedTips() []tip {
	if len(m.sessions) == 0 {
		return []tip{{"`+space", "create your first session"}}
	}

	tips := []tip{{"`+space", "focus topbar"}}
	if len(m.sessions) > 1 {
		tips = append(tips, tip{"`+1-9", "jump to session"})
		tips = append(tips, tip{"`+/", "search sessions"})
	}
	tips = append(tips, tip{"`+a", "launch agent"})

	// Suggest status dashboard when agents are running
	if m.hasActiveAgents() {
		tips = append(tips, tip{"`+s", "check agent status"})
	}
	return tips
}

// hasActiveAgents returns true if any session has a recent heartbeat.
func (m Model) hasActiveAgents() bool {
	for _, t := range m.agentActive {
		if time.Since(t) < constants.AgentIdleThreshold {
			return true
		}
	}
	return false
}

// focusedRepoTips builds tips for the repos row, conditionally showing archive tip.
func (m Model) focusedRepoTips() []tip {
	tips := make([]tip, len(tipsFocusedReposBase))
	copy(tips, tipsFocusedReposBase)
	if archived, _ := session.ListArchived(); len(archived) > 0 {
		tips = append(tips, tip{"A", "view archived"})
	}
	return tips
}

// focusedSessionTips builds tips for the sessions row based on active session state.
func (m Model) focusedSessionTips() []tip {
	tips := []tip{{"enter", "activate session"}}

	// Suggest adding a note if the selected session doesn't have one
	if name := m.selectedSessionName(); name != "" {
		for _, s := range m.sessions {
			if s.Name == name && s.Note == "" {
				tips = append(tips, tip{"N", "add a note"})
				break
			}
		}
	}

	tips = append(tips, tip{"s", "session status"})
	tips = append(tips, tip{"a", "archive session"})
	tips = append(tips, tip{"d", "delete session"})
	tips = append(tips, tip{"R", "rename session"})
	if archived, _ := session.ListArchived(); len(archived) > 0 {
		tips = append(tips, tip{"A", "view archived"})
	}
	return tips
}

// randomTipIdx returns a random index within the given tip slice.
func randomTipIdx(tips []tip) int {
	if len(tips) <= 1 {
		return 0
	}
	return rand.Intn(len(tips))
}

// renderTip formats a tip as "Tip: key description".
// Returns "" if the rendered tip exceeds maxWidth.
func renderTip(t tip, maxWidth int) string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.Focus).Bold(true)
	rendered := styles.HelpBar.Render("Tip:") + " " + keyStyle.Render(t.Key) + " " + styles.HelpBar.Render(t.Desc)
	if lipgloss.Width(rendered) > maxWidth {
		return ""
	}
	return rendered
}
