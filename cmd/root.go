package cmd

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/logging"
	"bay/internal/memory"
	baytmux "bay/internal/tmux"
	"bay/internal/tui"
)

// ensureValidTerm checks that the current TERM has a valid terminfo entry.
// Terminals like Ghostty set TERM=xterm-ghostty which requires a terminfo
// entry that may not be installed. Without it, tmux fails to start.
func ensureValidTerm() {
	term := os.Getenv("TERM")
	if term == "" || term == "xterm-256color" {
		return
	}
	// Use infocmp to check if the terminfo entry exists.
	if err := exec.Command("infocmp", term).Run(); err != nil {
		os.Setenv("TERM", "xterm-256color")
	}
}

// Root is the main `bay` command handler.
// If fresh is true, kills the existing bay session first.
func Root(fresh bool) error {
	logging.Init()
	logging.Info("bay starting (fresh=%v)", fresh)

	// Check tmux is installed
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux is required but not found. Install with: brew install tmux")
	}

	// If the current TERM lacks a terminfo entry, tmux will fail with
	// "server exited unexpectedly". Fall back to xterm-256color automatically.
	ensureValidTerm()

	if fresh && baytmux.SessionExists(baytmux.MainSession) {
		baytmux.KillMainSession()
	}

	firstRun := !config.Exists()

	if firstRun {
		if err := config.EnsureDirs(); err != nil {
			return fmt.Errorf("creating ~/.bay/: %w", err)
		}
	}

	// Get the bay binary path for the topbar command
	bayBin, err := os.Executable()
	if err != nil {
		bayBin = "bay"
	}

	// Create (or respawn) the bay tmux session with topbar layout
	logging.Info("creating main session with topbar cmd: %s --tui", bayBin)
	if err := baytmux.CreateMainSession(bayBin + " --tui"); err != nil {
		logging.Error("creating bay session: %v", err)
		return fmt.Errorf("creating bay session: %w", err)
	}

	// Set up tmux keybindings with configured agent command
	agentCmd := "claude"
	if cfg, err := config.Load(); err == nil && cfg.Defaults.Agent != "" {
		agentCmd = cfg.Defaults.Agent
	}
	baytmux.BindKeys(agentCmd)

	// If already inside tmux, switch client to bay session instead of attaching
	if os.Getenv("TMUX") != "" {
		cmd := exec.Command("tmux", "switch-client", "-t", baytmux.MainSession)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Attach to the bay session (blocks until detach/exit)
	cmd := exec.Command("tmux", "attach-session", "-t", baytmux.MainSession)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunTUIDirectly loads config and runs the TUI (for --tui flag).
func RunTUIDirectly() error {
	return runTUI()
}

// runTUI starts the Bubbletea app directly.
func runTUI() error {
	logging.Init()
	logging.Info("TUI starting (pid=%d)", os.Getpid())

	firstRun := !config.Exists()
	var cfg *config.Config

	if firstRun {
		logging.Info("first run — creating dirs and default config")
		if err := config.EnsureDirs(); err != nil {
			logging.Error("EnsureDirs: %v", err)
			return err
		}
		cfg = config.DefaultConfig()
	} else {
		var err error
		cfg, err = config.Load()
		if err != nil {
			logging.Error("config.Load: %v", err)
			return err
		}
	}

	// Process any pending LLM summaries from prior crashes/restarts
	go memory.ProcessPendingSummaries()

	app := tui.NewApp(cfg, firstRun)
	logging.Info("starting bubbletea program")
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus())
	if _, err := p.Run(); err != nil {
		logging.Error("bubbletea exited with error: %v", err)
		return err
	}

	logging.Info("TUI exited normally — killing main session")
	// When the TUI exits (q), kill the whole bay session
	baytmux.KillMainSession()
	return nil
}
