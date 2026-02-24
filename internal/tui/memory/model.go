package tmemory

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

	"bay/internal/memory"
)

type view int

const (
	viewOverview view = iota
	viewLog
	viewTask
	viewNote
)

// BackMsg signals return to topbar.
type BackMsg struct{}

// Model is the memory viewer screen state.
type Model struct {
	sessionName string
	working     *memory.WorkingState
	entries     []memory.EpisodicEntry
	pending     int
	view        view
	taskInput   textinput.Model
	noteInput   textinput.Model
	statusMsg   string
	width       int
	height      int
}

// New creates a new memory viewer for the given session.
func New(sessionName string) Model {
	ti := textinput.New()
	ti.Placeholder = "task description"
	ti.CharLimit = 200
	ti.Width = 50

	ni := textinput.New()
	ni.Placeholder = "note text"
	ni.CharLimit = 500
	ni.Width = 50

	m := Model{
		sessionName: sessionName,
		taskInput:   ti,
		noteInput:   ni,
	}
	m.loadData()
	return m
}

func (m *Model) loadData() {
	m.working, _ = memory.GetWorking(m.sessionName)
	m.entries, _ = memory.RecentEpisodic(m.sessionName, 15)
	m.pending, _ = memory.PendingSummaryCount()
}

// Init starts the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles input for the memory viewer.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Handle input modes first
		if m.view == viewTask {
			return m.updateTaskInput(msg)
		}
		if m.view == viewNote {
			return m.updateNoteInput(msg)
		}

		switch key {
		case "esc", "q", "m":
			return m, func() tea.Msg { return BackMsg{} }
		case "t":
			m.view = viewTask
			m.taskInput.Focus()
			if m.working != nil && m.working.CurrentTask != "" {
				m.taskInput.SetValue(m.working.CurrentTask)
			}
			return m, textinput.Blink
		case "n":
			m.view = viewNote
			m.noteInput.SetValue("")
			m.noteInput.Focus()
			return m, textinput.Blink
		case "l":
			if m.view == viewOverview {
				m.view = viewLog
			} else {
				m.view = viewOverview
			}
			return m, nil
		case "r":
			m.loadData()
			m.statusMsg = "Refreshed"
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateTaskInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		task := m.taskInput.Value()
		if task != "" {
			memory.SetTask(m.sessionName, task)
			m.statusMsg = "Task set"
			m.loadData()
		}
		m.view = viewOverview
		return m, nil
	case "esc":
		m.view = viewOverview
		return m, nil
	default:
		var cmd tea.Cmd
		m.taskInput, cmd = m.taskInput.Update(msg)
		return m, cmd
	}
}

func (m Model) updateNoteInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		note := m.noteInput.Value()
		if note != "" {
			memory.AppendEpisodic(m.sessionName, "note", note, "")
			m.statusMsg = "Note saved"
			m.loadData()
		}
		m.view = viewOverview
		return m, nil
	case "esc":
		m.view = viewOverview
		return m, nil
	default:
		var cmd tea.Cmd
		m.noteInput, cmd = m.noteInput.Update(msg)
		return m, cmd
	}
}
