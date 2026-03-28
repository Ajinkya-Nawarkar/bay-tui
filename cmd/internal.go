package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"bay/internal/config"
	"bay/internal/constants"
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

	case "ensure-pane":
		return internalEnsurePane()

	case "create":
		return InternalCreate(args[1:])

	case "agent-heartbeat":
		return internalAgentHeartbeat()

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

	buffer, err := baytmux.CapturePaneBuffer(paneID, constants.PaneCaptureBuffer)
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

	cfg, err := config.Load()
	if err != nil || cfg == nil {
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

func internalEnsurePane() error {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil // No active session — nothing to do
	}

	if err := baytmux.EnsureDevPane(s.TmuxWindow, s.WorkingDir); err != nil {
		return err
	}

	// Sync pane layout after spawning the new pane.
	hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
	return nil
}

// internalAgentHeartbeat writes the current timestamp to the agent status file.
// Reads BAY_SESSION env var set by `bay agent` at launch. Called by the
// PreToolUse Claude Code hook. Near-instant — just reads an env var and writes a file.
func internalAgentHeartbeat() error {
	sessionName := os.Getenv("BAY_SESSION")
	if sessionName == "" {
		return nil // Not launched via bay agent
	}
	home, _ := os.UserHomeDir()
	dir := home + "/.bay/agent-status"
	os.MkdirAll(dir, 0o755)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	os.WriteFile(dir+"/"+sessionName, []byte(ts), 0o644)
	return nil
}

func internalSyncPanes() error {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil // No active session — nothing to sync
	}

	hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
	return nil
}
