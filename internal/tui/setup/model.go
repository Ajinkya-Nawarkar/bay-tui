package setup

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ajinkya-Nawarkar/bay-tui/internal/config"
)

type step int

const (
	stepWelcome step = iota
	stepScanDir
	stepWorktreeLocation
	stepDone
)

// DoneMsg is sent when setup is complete.
type DoneMsg struct {
	Config *config.Config
}

// Model is the setup wizard state.
type Model struct {
	step             step
	scanDirInput     textinput.Model
	worktreeChoice   int // 0 = managed, 1 = adjacent
	cfg              *config.Config
	err              error
}

// New creates a new setup wizard model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "/Users/you/workspace"
	ti.CharLimit = 256
	ti.Width = 50

	// Pre-fill with ~/workspace if it exists
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, "workspace")
	if _, err := os.Stat(defaultDir); err == nil {
		ti.SetValue(defaultDir)
	}

	ti.Focus()

	return Model{
		step:         stepWelcome,
		scanDirInput: ti,
		cfg:          config.DefaultConfig(),
	}
}

// Init starts the text input blink.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input for the setup wizard.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepWelcome:
			if msg.String() == "enter" {
				m.step = stepScanDir
				return m, textinput.Blink
			}
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

		case stepScanDir:
			switch msg.String() {
			case "enter":
				dir := m.scanDirInput.Value()
				if dir != "" {
					m.cfg.ScanDirs = []string{dir}
				}
				m.step = stepWorktreeLocation
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				var cmd tea.Cmd
				m.scanDirInput, cmd = m.scanDirInput.Update(msg)
				return m, cmd
			}

		case stepWorktreeLocation:
			switch msg.String() {
			case "1":
				m.cfg.Defaults.WorktreeLocation = "managed"
				m.worktreeChoice = 0
			case "2":
				m.cfg.Defaults.WorktreeLocation = "adjacent"
				m.worktreeChoice = 1
			case "enter":
				// Save config
				if err := config.Save(m.cfg); err != nil {
					m.err = err
					return m, nil
				}
				m.step = stepDone
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}

		case stepDone:
			if msg.String() == "enter" || msg.String() == "q" {
				return m, func() tea.Msg {
					return DoneMsg{Config: m.cfg}
				}
			}
		}
	}

	return m, nil
}
