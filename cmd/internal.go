package cmd

import (
	"fmt"
	"os"
	"strings"

	"bay/internal/config"
	"bay/internal/hooks"
	"bay/internal/memory"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
)

// Internal handles `bay internal` subcommands — tmux hooks and plumbing.
// Not intended for direct user use.
func Internal(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "bay internal — tmux hook plumbing (not for direct use)")
		return nil
	}

	switch args[0] {
	case "capture":
		if len(args) < 2 {
			return fmt.Errorf("usage: bay internal capture <pane-id>")
		}
		return internalCapture(args[1])

	case "record":
		if len(args) < 4 {
			return fmt.Errorf("usage: bay internal record <type> <pane-id> <data>")
		}
		return internalRecord(args[1], args[2], strings.Join(args[3:], " "))

	case "sync-panes":
		return internalSyncPanes()

	default:
		fmt.Fprintf(os.Stderr, "Unknown internal command: %s\n", args[0])
		return nil
	}
}

func internalCapture(paneID string) error {
	sessions, err := session.List()
	if err != nil {
		return err
	}

	buffer, err := baytmux.CapturePaneBuffer(paneID, 100)
	if err != nil {
		return fmt.Errorf("capturing pane %s: %w", paneID, err)
	}

	if len(strings.TrimSpace(buffer)) == 0 {
		return nil
	}

	sessionName := "unknown"
	for _, s := range sessions {
		if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
			sessionName = s.Name
			break
		}
	}

	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if cfg.Memory.AutoSummarize {
		return memory.SummarizeAsync(sessionName, buffer, paneID)
	}

	return memory.AppendEpisodic(sessionName, "pane_snapshot", buffer, paneID)
}

func internalRecord(eventType, paneID, data string) error {
	s, err := session.FindActiveSession()
	if err != nil {
		return memory.AppendEpisodic("unknown", eventType, data, paneID)
	}
	return memory.AppendEpisodic(s.Name, eventType, data, paneID)
}

func internalSyncPanes() error {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil // No active session — nothing to sync
	}

	hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
	return nil
}
