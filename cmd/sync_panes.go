package cmd

import (
	"bay/internal/hooks"
	"bay/internal/session"
)

// SyncPanes snapshots the current window's pane layout to the active session YAML.
// Called from tmux hooks (after-split-window, after-kill-pane) so pane state is
// persisted immediately rather than only on session deactivation.
func SyncPanes() error {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil // No active session — nothing to sync
	}

	hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
	return nil
}
