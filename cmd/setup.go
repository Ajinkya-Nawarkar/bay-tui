package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anawarkar/bay/internal/config"
	"github.com/anawarkar/bay/internal/tui/setup"
)

// Setup runs the setup wizard standalone.
func Setup() error {
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("creating ~/.bay/: %w", err)
	}

	m := setup.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
