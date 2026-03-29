package purpose

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/memory"
	"bay/internal/session"
)

// DoneMsg signals the view is closing.
type DoneMsg struct{}

// CancelMsg signals the user cancelled.
type CancelMsg struct{}

type mode int

const (
	modeBrowse mode = iota
	modeEditPurpose
	modeAddItem
)

// Model is the purpose view state.
type Model struct {
	session *session.Session
	purpose string
	items   []memory.Task
	cursor  int
	mode    mode
	input   textinput.Model
	width   int
	height  int
}

// New creates a purpose view for the active session.
func New() Model {
	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 60

	m := Model{input: ti}
	m.loadData()
	return m
}

// Init starts the TUI.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeEditPurpose:
			return m.updateEditPurpose(msg)
		case modeAddItem:
			return m.updateAddItem(msg)
		}
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "ctrl+c":
		return m, func() tea.Msg { return DoneMsg{} }

	case "e":
		m.mode = modeEditPurpose
		m.input.SetValue(m.purpose)
		m.input.Focus()
		m.input.Placeholder = "session purpose..."
		return m, textinput.Blink

	case "a":
		m.mode = modeAddItem
		m.input.SetValue("")
		m.input.Focus()
		m.input.Placeholder = "checklist item..."
		return m, textinput.Blink

	case "d":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			newStatus := "done"
			if item.Status == "done" {
				newStatus = "todo"
			}
			memory.SetTaskStatus(item.ID, newStatus)
			m.reloadItems()
		}
		return m, nil

	case "x":
		if m.cursor < len(m.items) {
			memory.DeleteTask(m.items[m.cursor].ID)
			m.reloadItems()
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor--
			}
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateEditPurpose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.purpose = m.input.Value()
		if m.session != nil {
			m.session.Purpose = m.purpose
			session.Save(m.session)
		}
		m.mode = modeBrowse
		return m, nil
	case "esc":
		m.mode = modeBrowse
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateAddItem(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		title := m.input.Value()
		if title != "" && m.session != nil {
			memory.CreateTask(m.session.Name, title)
			m.reloadItems()
			m.cursor = len(m.items) - 1
		}
		m.mode = modeBrowse
		return m, nil
	case "esc":
		m.mode = modeBrowse
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) loadData() {
	s, err := session.FindActiveSession()
	if err != nil {
		return
	}
	m.session = s
	m.purpose = s.Purpose
	m.reloadItems()
}

func (m *Model) reloadItems() {
	if m.session == nil {
		return
	}
	m.items, _ = memory.ListTasks(m.session.Name)
}
