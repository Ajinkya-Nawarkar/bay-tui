// Package tui implements the Bubbletea TUI layer for bay.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/tui/setup"
	"bay/internal/tui/topbar"
)

type screen int

const (
	screenTopbar screen = iota
	screenSetup
)

// App is the root Bubbletea model that switches between screens.
type App struct {
	screen     screen
	topbar     topbar.Model
	setupModel setup.Model
	cfg        *config.Config
	firstRun   bool
}

// NewApp creates the root app model.
func NewApp(cfg *config.Config, firstRun bool) App {
	a := App{
		cfg:      cfg,
		firstRun: firstRun,
	}

	if firstRun {
		a.screen = screenSetup
		a.setupModel = setup.New()
	} else {
		a.screen = screenTopbar
		a.topbar = topbar.New(cfg)
	}

	return a
}

// Init starts the app.
func (a App) Init() tea.Cmd {
	switch a.screen {
	case screenSetup:
		return a.setupModel.Init()
	case screenTopbar:
		return a.topbar.Init()
	}
	return nil
}

// Update routes messages to the active screen.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global: Ctrl+C always quits from any screen
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		return a, tea.Quit
	}

	switch msg := msg.(type) {
	case setup.DoneMsg:
		a.cfg = msg.Config
		a.screen = screenTopbar
		a.topbar = topbar.New(a.cfg)
		return a, nil

	case topbar.SwitchToSetupMsg:
		a.setupModel = setup.New()
		a.screen = screenSetup
		return a, a.setupModel.Init()
	}

	// Route to active screen
	switch a.screen {
	case screenTopbar:
		m, cmd := a.topbar.Update(msg)
		a.topbar = m.(topbar.Model)
		return a, cmd

	case screenSetup:
		m, cmd := a.setupModel.Update(msg)
		a.setupModel = m.(setup.Model)
		return a, cmd
	}

	return a, nil
}

// View renders the active screen.
func (a App) View() string {
	switch a.screen {
	case screenSetup:
		return a.setupModel.View()
	default:
		return a.topbar.View()
	}
}
