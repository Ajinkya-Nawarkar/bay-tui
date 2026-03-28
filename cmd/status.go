package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/session"
	"bay/internal/tui/status"
)

// InternalStatus runs the status dashboard TUI as a standalone Bubbletea program.
func InternalStatus() error {
	m := status.New()
	wrapper := statusWrapper{inner: m}
	p := tea.NewProgram(wrapper, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	w := finalModel.(statusWrapper)
	if w.cancelled {
		_ = session.ClearSwitchTarget()
		os.Exit(1)
	}
	return nil
}

type statusWrapper struct {
	inner     status.Model
	cancelled bool
}

func (w statusWrapper) Init() tea.Cmd {
	return w.inner.Init()
}

func (w statusWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		w.cancelled = true
		return w, tea.Quit
	}

	switch msg.(type) {
	case status.DoneMsg:
		dm := msg.(status.DoneMsg)
		if dm.SessionName != "" {
			_ = session.SaveSwitchTarget(dm.SessionName)
		}
		return w, tea.Quit
	case status.CancelMsg:
		w.cancelled = true
		return w, tea.Quit
	}

	m, cmd := w.inner.Update(msg)
	w.inner = m.(status.Model)
	return w, cmd
}

func (w statusWrapper) View() string {
	return w.inner.View()
}
