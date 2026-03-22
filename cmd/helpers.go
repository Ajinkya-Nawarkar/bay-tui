package cmd

import (
	"fmt"

	"bay/internal/constants"
	"bay/internal/session"
)

// resolveSessionName resolves an optional session name to the active session.
// If name is non-empty, returns it as-is. Otherwise detects the active session
// from the current tmux window.
func resolveSessionName(name string) (string, error) {
	if name != "" {
		return name, nil
	}
	s, err := session.FindActiveSession()
	if err != nil {
		return "", fmt.Errorf("no active session (specify one): %w", err)
	}
	return s.Name, nil
}

// truncateContent truncates s to maxLen characters, appending "..." if needed.
func truncateContent(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// truncatePreview truncates s to the default ContentTruncateLen.
func truncatePreview(s string) string {
	return truncateContent(s, constants.ContentTruncateLen)
}
