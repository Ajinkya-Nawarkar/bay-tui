package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bay/internal/config"
	baytmux "bay/internal/tmux"
)

// Uninstall removes all bay data and cleans up the Claude hook.
func Uninstall() error {
	// Kill bay tmux session if running
	if baytmux.SessionExists(baytmux.MainSession) {
		baytmux.KillMainSession()
		fmt.Println("Killed bay tmux session.")
	}

	// Remove ~/.bay/
	bayDir := config.BayDir()
	if _, err := os.Stat(bayDir); err == nil {
		if err := os.RemoveAll(bayDir); err != nil {
			return fmt.Errorf("removing %s: %w", bayDir, err)
		}
		fmt.Printf("Removed %s\n", bayDir)
	}

	// Remove bay context hook from ~/.claude/settings.json
	if removed := removeClaudeHook(); removed {
		fmt.Println("Removed bay context hook from ~/.claude/settings.json")
	}

	fmt.Println("Bay uninstalled.")
	return nil
}

// removeClaudeHook removes the bay context SessionStart hook from Claude settings.
func removeClaudeHook() bool {
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}

	startHooks, ok := hooks["SessionStart"].([]any)
	if !ok {
		return false
	}

	// Filter out any hook entry that contains "bay context"
	var filtered []any
	removed := false
	for _, item := range startHooks {
		m, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		innerHooks, ok := m["hooks"].([]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		isBay := false
		for _, h := range innerHooks {
			if hm, ok := h.(map[string]any); ok {
				if cmd, _ := hm["command"].(string); strings.HasSuffix(cmd, "bay context") {
					isBay = true
					break
				}
			}
		}
		if isBay {
			removed = true
		} else {
			filtered = append(filtered, item)
		}
	}

	if !removed {
		return false
	}

	if len(filtered) == 0 {
		delete(hooks, "SessionStart")
	} else {
		hooks["SessionStart"] = filtered
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false
	}
	os.WriteFile(settingsPath, out, 0644)
	return true
}
