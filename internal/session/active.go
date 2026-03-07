package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"bay/internal/config"
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

func activeSessionPath() string {
	return filepath.Join(config.BayDir(), ".active-session")
}

// SaveActiveSession persists the active session name to disk.
func SaveActiveSession(name string) {
	os.WriteFile(activeSessionPath(), []byte(name), 0644)
}

// LoadActiveSession reads the last active session name from disk.
func LoadActiveSession() string {
	data, err := os.ReadFile(activeSessionPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
