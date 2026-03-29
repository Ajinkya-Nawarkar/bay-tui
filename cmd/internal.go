package cmd

import (
	"fmt"
	"os"
	"time"

	"bay/internal/hooks"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
)

// Internal handles `bay internal` subcommands — tmux hooks and plumbing.
func Internal(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "bay internal — tmux hook plumbing (not for direct use)")
		return nil
	}

	switch args[0] {
	case "sync-panes":
		return internalSyncPanes()

	case "ensure-pane":
		return internalEnsurePane()

	case "create":
		return InternalCreate(args[1:])

	case "archive":
		return InternalArchive()

	case "agent-heartbeat":
		return internalAgentHeartbeat()

	case "search":
		return InternalSearch()

	case "purpose":
		return InternalPurpose()

	default:
		fmt.Fprintf(os.Stderr, "Unknown internal command: %s\n", args[0])
		return nil
	}
}

func internalEnsurePane() error {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil
	}

	if err := baytmux.EnsureDevPane(s.TmuxWindow, s.WorkingDir); err != nil {
		return err
	}

	hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
	return nil
}

func internalAgentHeartbeat() error {
	sessionName := os.Getenv("BAY_SESSION")
	if sessionName == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	dir := home + "/.bay/agent-status"
	os.MkdirAll(dir, 0o755)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	os.WriteFile(dir+"/"+sessionName, []byte(ts), 0o644)
	return nil
}

func internalSyncPanes() error {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil
	}

	hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
	return nil
}
