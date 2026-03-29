package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"bay/internal/config"
	"bay/internal/constants"
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
	return filepath.Join(config.BayDir(), constants.ActiveSessionFile)
}

// SaveActiveSession persists the active session name to disk.
func SaveActiveSession(name string) error {
	return os.WriteFile(activeSessionPath(), []byte(name), 0o644)
}

// LoadActiveSession reads the last active session name from disk.
func LoadActiveSession() string {
	data, err := os.ReadFile(activeSessionPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func createdSessionPath() string {
	return filepath.Join(config.BayDir(), constants.CreatedSessionFile)
}

// SaveCreatedSession persists the name of a just-created session to disk.
func SaveCreatedSession(name string) error {
	return os.WriteFile(createdSessionPath(), []byte(name), 0o644)
}

// LoadCreatedSession reads the created session name from disk.
func LoadCreatedSession() string {
	data, err := os.ReadFile(createdSessionPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ClearCreatedSession removes the created-session marker file.
func ClearCreatedSession() error {
	err := os.Remove(createdSessionPath())
	if os.IsNotExist(err) {
		return nil // already gone
	}
	return err
}

func switchTargetPath() string {
	return filepath.Join(config.BayDir(), constants.SwitchTargetFile)
}

// SaveSwitchTarget persists the selected session name for the topbar to read.
func SaveSwitchTarget(name string) error {
	return os.WriteFile(switchTargetPath(), []byte(name), 0o644)
}

// LoadSwitchTarget reads the switch target session name from disk.
func LoadSwitchTarget() string {
	data, err := os.ReadFile(switchTargetPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ClearSwitchTarget removes the switch-target marker file.
func ClearSwitchTarget() error {
	err := os.Remove(switchTargetPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
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
