package cmd

import (
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/tui/purpose"
)

// InternalPurpose runs the purpose view TUI as a standalone Bubbletea program.
func InternalPurpose() error {
	m := purpose.New()
	wrapper := purposeWrapper{inner: m}
	p := tea.NewProgram(wrapper, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type purposeWrapper struct {
	inner purpose.Model
}

func (w purposeWrapper) Init() tea.Cmd {
	return w.inner.Init()
}

func (w purposeWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		return w, tea.Quit
	}

	switch msg.(type) {
	case purpose.DoneMsg, purpose.CancelMsg:
		return w, tea.Quit
	}

	m, cmd := w.inner.Update(msg)
	w.inner = m.(purpose.Model)
	return w, cmd
}

func (w purposeWrapper) View() string {
	return w.inner.View()
}
