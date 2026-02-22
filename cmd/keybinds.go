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
	fmt.Println("Top bar (always available via ` prefix):")
	fmt.Println("  `+Tab             Cycle to next session")
	fmt.Println("  `+0-9             Jump to session by index")
	fmt.Println("  `+r               Cycle to next repo")
	fmt.Println("  `+q               Toggle bar focus mode")
	fmt.Println()
	fmt.Println("Top bar (focused mode — `+q to enter, esc to leave):")
	fmt.Println("  h / l             Switch repo left/right")
	fmt.Println("  n / d / R         New / delete / rename session")
	fmt.Println("  Enter             Activate session")
	fmt.Println("  esc               Leave focus mode")
	fmt.Println()
	fmt.Println("Pane management (` then):")
	fmt.Println("  Arrow             Navigate panes")
	fmt.Println("  d                 Vertical split")
	fmt.Println("  D                 Horizontal split")
	fmt.Println("  w                 Close pane")
	fmt.Println("  s                 Toggle topbar/dev focus")
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
