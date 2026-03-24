package cmd

import (
	"fmt"
	"os"

	"bay/internal/config"
	"bay/internal/constants"
	"bay/internal/hooks"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
)

// Refresh re-syncs pane layouts for all sessions and restarts the topbar TUI.
func Refresh() error {
	if !baytmux.SessionExists(baytmux.MainSession) {
		return fmt.Errorf("bay session not running — start with: bay")
	}

	// Re-sync pane layouts for all sessions with live windows
	sessions, _ := session.List()
	synced := 0
	for _, s := range sessions {
		if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
			hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
			synced++
		}
	}
	fmt.Printf("Synced %d session(s)\n", synced)

	// Re-bind all keybindings
	bayBin, err := os.Executable()
	if err != nil {
		bayBin = "bay"
	}
	baytmux.SetBayBin(bayBin)

	agentCmd := constants.DefaultAgent
	if cfg, err := config.Load(); err == nil && cfg.Defaults.Agent != "" {
		agentCmd = cfg.Defaults.Agent
	}
	baytmux.BindKeys(agentCmd)
	fmt.Println("Rebound keybindings")

	// Restart topbar via respawn-pane (triggers the bash restart loop)
	topbarCmd := bayBin + " --tui"
	if err := baytmux.RestartTopbar(topbarCmd); err != nil {
		return fmt.Errorf("restarting topbar: %w", err)
	}
	fmt.Println("Topbar restarted")

	return nil
}
