package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anawarkar/bay/internal/config"
	"github.com/anawarkar/bay/internal/scanner"
	"github.com/anawarkar/bay/internal/tui/create"
	"github.com/anawarkar/bay/internal/tui/setup"
	"github.com/anawarkar/bay/internal/tui/sidebar"
)

type screen int

const (
	screenSidebar screen = iota
	screenSetup
	screenCreate
)

// App is the root Bubbletea model that switches between screens.
type App struct {
	screen       screen
	sidebar      sidebar.Model
	setupModel   setup.Model
	createModel  create.Model
	cfg          *config.Config
	firstRun     bool
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
		a.screen = screenSidebar
		a.sidebar = sidebar.New(cfg)
	}

	return a
}

// Init starts the app.
func (a App) Init() tea.Cmd {
	switch a.screen {
	case screenSetup:
		return a.setupModel.Init()
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
	// Screen switching messages
	case setup.DoneMsg:
		a.cfg = msg.Config
		a.screen = screenSidebar
		a.sidebar = sidebar.New(a.cfg)
		return a, nil

	case sidebar.SwitchToCreateMsg:
		repos := scanner.Scan(a.cfg.ScanDirs)
		a.createModel = create.New(repos, msg.PreselectedRepo)
		a.screen = screenCreate
		return a, a.createModel.Init()

	case sidebar.SwitchToSetupMsg:
		a.setupModel = setup.New()
		a.screen = screenSetup
		return a, a.setupModel.Init()

	case create.DoneMsg:
		a.screen = screenSidebar
		a.sidebar.Refresh()
		if msg.Session != nil {
			a.sidebar.ActivateSession(msg.Session.Name)
			a.sidebar.SetStatus("Created '" + msg.Session.Name + "'")
		}
		return a, tea.ClearScreen

	case create.CancelMsg:
		a.screen = screenSidebar
		return a, nil
	}

	// Route to active screen
	switch a.screen {
	case screenSidebar:
		m, cmd := a.sidebar.Update(msg)
		a.sidebar = m.(sidebar.Model)
		return a, cmd

	case screenSetup:
		m, cmd := a.setupModel.Update(msg)
		a.setupModel = m.(setup.Model)
		return a, cmd

	case screenCreate:
		// Handle async create messages
		switch msg.(type) {
		case create.DoneMsg, create.CancelMsg:
			// Already handled above
		default:
			m, cmd := a.createModel.Update(msg)
			a.createModel = m.(create.Model)
			return a, cmd
		}
	}

	return a, nil
}

// View renders the active screen.
func (a App) View() string {
	switch a.screen {
	case screenSetup:
		return a.setupModel.View()
	case screenCreate:
		return a.createModel.View()
	default:
		return a.sidebar.View()
	}
}
