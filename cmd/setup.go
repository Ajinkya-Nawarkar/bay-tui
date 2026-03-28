package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/tui/setup"
)

// setupWrapper wraps the setup model to quit on DoneMsg when running standalone.
type setupWrapper struct {
	inner tea.Model
}

func (w setupWrapper) Init() tea.Cmd                           { return w.inner.Init() }
func (w setupWrapper) View() string                            { return w.inner.View() }
func (w setupWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(setup.DoneMsg); ok {
		return w, tea.Quit
	}
	m, cmd := w.inner.Update(msg)
	return setupWrapper{inner: m}, cmd
}

// Setup runs the setup wizard standalone.
func Setup() error {
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("creating ~/.bay/: %w", err)
	}

	m := setupWrapper{inner: setup.New()}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
