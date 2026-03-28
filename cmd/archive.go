package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/tui/archive"
)

// InternalArchive runs the archive browser as a standalone Bubbletea program.
// Called by `bay internal archive`.
func InternalArchive() error {
	m := archive.New()
	wrapper := archiveWrapper{inner: m}
	p := tea.NewProgram(wrapper, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	w := finalModel.(archiveWrapper)
	if w.cancelled {
		os.Exit(1)
	}
	return nil
}

type archiveWrapper struct {
	inner     archive.Model
	cancelled bool
}

func (w archiveWrapper) Init() tea.Cmd {
	return w.inner.Init()
}

func (w archiveWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		w.cancelled = true
		return w, tea.Quit
	}

	switch msg.(type) {
	case archive.DoneMsg:
		return w, tea.Quit
	case archive.CancelMsg:
		w.cancelled = true
		return w, tea.Quit
	}

	m, cmd := w.inner.Update(msg)
	w.inner = m.(archive.Model)
	return w, cmd
}

func (w archiveWrapper) View() string {
	return w.inner.View()
}
