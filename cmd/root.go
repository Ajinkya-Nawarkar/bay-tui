package cmd

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anawarkar/bay/internal/config"
	baytmux "github.com/anawarkar/bay/internal/tmux"
	"github.com/anawarkar/bay/internal/tui"
)

// Root is the main `bay` command handler.
func Root() error {
	// Check tmux is installed
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux is required but not found. Install with: brew install tmux")
	}

	firstRun := !config.Exists()

	if firstRun {
		if err := config.EnsureDirs(); err != nil {
			return fmt.Errorf("creating ~/.bay/: %w", err)
		}
	}

	// If we're already inside tmux, run the TUI directly (--tui mode)
	if os.Getenv("TMUX") != "" {
		return runTUI()
	}

	// Get the bay binary path for the sidebar command
	bayBin, err := os.Executable()
	if err != nil {
		bayBin = "bay"
	}

	// Create the bay tmux session with sidebar layout if it doesn't exist
	if err := baytmux.CreateMainSession(bayBin + " --tui"); err != nil {
		return fmt.Errorf("creating bay session: %w", err)
	}

	// Set up tmux keybindings
	baytmux.BindKeys()

	// Attach to the bay session (blocks until detach/exit)
	cmd := exec.Command("tmux", "attach-session", "-t", baytmux.MainSession)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	attachErr := cmd.Run()

	return attachErr
}

// RunTUIDirectly loads config and runs the TUI (for --tui flag).
func RunTUIDirectly() error {
	return runTUI()
}

// runTUI starts the Bubbletea app directly.
func runTUI() error {
	firstRun := !config.Exists()
	var cfg *config.Config

	if firstRun {
		if err := config.EnsureDirs(); err != nil {
			return err
		}
		cfg = config.DefaultConfig()
	} else {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
	}

	app := tui.NewApp(cfg, firstRun)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	// When the TUI exits (q), kill the whole bay session
	baytmux.KillMainSession()
	return nil
}
