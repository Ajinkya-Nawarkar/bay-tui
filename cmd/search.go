package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/session"
	"bay/internal/tui/search"
)

// InternalSearch runs the search TUI as a standalone Bubbletea program.
func InternalSearch() error {
	m := search.New()
	wrapper := searchWrapper{inner: m}
	p := tea.NewProgram(wrapper, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	w := finalModel.(searchWrapper)
	if w.cancelled {
		_ = session.ClearSwitchTarget()
		os.Exit(1)
	}
	return nil
}

type searchWrapper struct {
	inner     search.Model
	cancelled bool
}

func (w searchWrapper) Init() tea.Cmd {
	return w.inner.Init()
}

func (w searchWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		w.cancelled = true
		return w, tea.Quit
	}

	switch msg.(type) {
	case search.DoneMsg:
		dm := msg.(search.DoneMsg)
		if dm.SessionName != "" {
			_ = session.SaveSwitchTarget(dm.SessionName)
		}
		return w, tea.Quit
	case search.CancelMsg:
		w.cancelled = true
		return w, tea.Quit
	}

	m, cmd := w.inner.Update(msg)
	w.inner = m.(search.Model)
	return w, cmd
}

func (w searchWrapper) View() string {
	return w.inner.View()
}
