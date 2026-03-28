package archive

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/hooks"
	"bay/internal/session"
	"bay/internal/worktree"
)

// DoneMsg signals the archive view exited with changes.
type DoneMsg struct{}

// CancelMsg signals the archive view was dismissed without changes.
type CancelMsg struct{}

// Model is the archive browser state.
type Model struct {
	sessions  []*session.Session // all archived
	filtered  []*session.Session // after search filter
	cursor    int
	searching bool
	search    textinput.Model
	statusMsg string
	changed   bool // true if any unarchive/delete happened
	width     int
	height    int
}

// New creates a new archive browser, loading all archived sessions.
func New() Model {
	si := textinput.New()
	si.Placeholder = "filter sessions..."
	si.CharLimit = 100
	si.Width = 40

	archived, _ := session.ListArchived()

	m := Model{
		sessions: archived,
		search:   si,
	}
	m.filtered = m.sessions
	return m
}

// Init starts the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles input for the archive browser.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if m.changed {
			return m, func() tea.Msg { return DoneMsg{} }
		}
		return m, func() tea.Msg { return CancelMsg{} }

	case "/":
		m.searching = true
		m.search.Focus()
		return m, textinput.Blink

	case "u":
		if m.cursor < len(m.filtered) {
			s := m.filtered[m.cursor]
			session.Unarchive(s.Name)
			m.statusMsg = "Unarchived '" + s.Name + "'"
			m.changed = true
			m.reload()
		}
		return m, nil

	case "d":
		if m.cursor < len(m.filtered) {
			s := m.filtered[m.cursor]
			if s.IsWorktree && s.WorktreeBranch != "" {
				worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch)
			}
			hooks.OnSessionDelete(s.Name)
			session.Delete(s.Name)
			m.statusMsg = "Deleted '" + s.Name + "'"
			m.changed = true
			m.reload()
		}
		if len(m.filtered) == 0 {
			return m, func() tea.Msg { return DoneMsg{} }
		}
		return m, nil

	case "U":
		for _, s := range m.sessions {
			session.Unarchive(s.Name)
		}
		m.statusMsg = "Unarchived all sessions"
		m.changed = true
		m.sessions = nil
		m.filtered = nil
		return m, func() tea.Msg { return DoneMsg{} }

	case "D":
		for _, s := range m.sessions {
			if s.IsWorktree && s.WorktreeBranch != "" {
				worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch)
			}
			hooks.OnSessionDelete(s.Name)
			session.Delete(s.Name)
		}
		m.changed = true
		m.sessions = nil
		m.filtered = nil
		return m, func() tea.Msg { return DoneMsg{} }

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.SetValue("")
		m.search.Blur()
		m.filtered = m.sessions
		m.cursor = 0
		return m, nil
	case "enter":
		m.searching = false
		m.search.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.applyFilter()
		return m, cmd
	}
}

func (m *Model) applyFilter() {
	query := strings.ToLower(m.search.Value())
	if query == "" {
		m.filtered = m.sessions
		m.cursor = 0
		return
	}
	var out []*session.Session
	for _, s := range m.sessions {
		label := strings.ToLower(s.Repo + "/" + s.Name)
		if strings.Contains(label, query) {
			out = append(out, s)
		}
	}
	m.filtered = out
	m.cursor = 0
}

func (m *Model) reload() {
	m.sessions, _ = session.ListArchived()
	m.applyFilter()
	if m.cursor >= len(m.filtered) && m.cursor > 0 {
		m.cursor = len(m.filtered) - 1
	}
}
