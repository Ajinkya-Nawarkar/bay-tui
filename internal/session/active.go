package session

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FindActiveSession detects the active session from the current tmux window.
// It reads the current window index from tmux and matches it against saved sessions.
func FindActiveSession() (*Session, error) {
	windowIdx, err := currentWindowIndex()
	if err != nil {
		return nil, fmt.Errorf("detecting tmux window: %w", err)
	}

	sessions, err := List()
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	for _, s := range sessions {
		if s.TmuxWindow == windowIdx {
			return s, nil
		}
	}

	return nil, fmt.Errorf("no bay session found for window %d", windowIdx)
}

// currentWindowIndex returns the active tmux window index.
func currentWindowIndex() (int, error) {
	cmd := exec.Command("tmux", "display-message", "-p", "#{window_index}")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("not in a tmux session: %w", err)
	}
	idx, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parsing window index: %w", err)
	}
	return idx, nil
}
