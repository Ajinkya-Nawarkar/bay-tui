package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Keybinds prints the keybind reference and terminal setup tips.
func Keybinds() error {
	fmt.Println("bay — Keybind Reference")
	fmt.Println()
	fmt.Println("Sidebar (when focused):")
	fmt.Println("  c                 Add Claude Code pane")
	fmt.Println("  n / d / r         New / delete / rename session")
	fmt.Println("  Enter             Switch to session")
	fmt.Println("  j/k               Navigate")
	fmt.Println("  Tab               Expand/collapse repo")
	fmt.Println("  s                 Re-run setup")
	fmt.Println("  q                 Quit")
	fmt.Println()
	fmt.Println("Pane management (` then):")
	fmt.Println("  Arrow             Navigate panes")
	fmt.Println("  D                 Vertical split")
	fmt.Println("  Shift+D           Horizontal split")
	fmt.Println("  W                 Close pane")
	fmt.Println("  S                 Toggle sidebar/dev focus")
	fmt.Println("  Click             Focus any pane (mouse)")
	fmt.Println()
	fmt.Println("  ``                Type a literal backtick")
	fmt.Println("  Ctrl+B also works as the standard tmux prefix.")

	// Check for leftover Ghostty bay keybinds block and offer to clean it up
	if cleaned := cleanGhosttyConfig(); cleaned {
		fmt.Println()
		fmt.Println("Cleaned up leftover bay keybinds block from Ghostty config.")
	}

	return nil
}

// cleanGhosttyConfig removes any leftover bay keybind block from the Ghostty config.
// Returns true if a block was found and removed.
func cleanGhosttyConfig() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cfgPath := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty", "config")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return false
	}

	content := string(data)
	const marker = "# >>> bay keybinds >>>"
	const endMarker = "# <<< bay keybinds <<<"

	if !strings.Contains(content, marker) {
		return false
	}

	startIdx := strings.Index(content, marker)
	endIdx := strings.Index(content, endMarker)
	if startIdx == -1 || endIdx == -1 {
		return false
	}

	endIdx += len(endMarker)
	before := strings.TrimRight(content[:startIdx], "\n")
	after := strings.TrimLeft(content[endIdx:], "\n")

	var cleaned string
	if after != "" {
		cleaned = before + "\n" + after
	} else {
		cleaned = before + "\n"
	}

	if err := os.WriteFile(cfgPath, []byte(cleaned), 0644); err != nil {
		return false
	}
	return true
}
