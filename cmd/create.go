package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/scanner"
	"bay/internal/session"
	"bay/internal/tui/create"
)

// InternalCreate runs the create session wizard as a standalone Bubbletea program.
// Called by `bay internal create [--repo=X]`.
func InternalCreate(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var preselectedRepo string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--repo=") {
			preselectedRepo = strings.TrimPrefix(arg, "--repo=")
		}
	}

	repos := scanner.Scan(cfg.ScanDirs)

	// Build set of repos that already have sessions
	sessions, _ := session.List()
	reposWithSessions := make(map[string]bool)
	for _, s := range sessions {
		reposWithSessions[s.Repo] = true
	}

	m := create.NewWithSessionInfo(repos, reposWithSessions, preselectedRepo)

	// Wrap in a standalone app that intercepts Done/Cancel
	wrapper := createWrapper{inner: m}
	p := tea.NewProgram(wrapper, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	w := finalModel.(createWrapper)
	if w.cancelled {
		session.ClearCreatedSession()
		os.Exit(1)
	}
	return nil
}

// createWrapper wraps the create.Model to intercept DoneMsg/CancelMsg.
type createWrapper struct {
	inner     create.Model
	cancelled bool
}

func (w createWrapper) Init() tea.Cmd {
	return w.inner.Init()
}

func (w createWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global Ctrl+C
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		w.cancelled = true
		return w, tea.Quit
	}

	switch msg.(type) {
	case create.DoneMsg:
		dm := msg.(create.DoneMsg)
		if dm.Session != nil {
			session.SaveCreatedSession(dm.Session.Name)
		}
		return w, tea.Quit
	case create.CancelMsg:
		w.cancelled = true
		return w, tea.Quit
	}

	m, cmd := w.inner.Update(msg)
	w.inner = m.(create.Model)
	return w, cmd
}

func (w createWrapper) View() string {
	return w.inner.View()
}
