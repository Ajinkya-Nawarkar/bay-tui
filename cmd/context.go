package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"bay/internal/config"
	"bay/internal/memory"
	"bay/internal/session"
)

// Context outputs session context to stdout for use as a Claude SessionStart hook.
// This must NEVER return an error — a non-zero exit causes Claude to show "startup hook error".
func Context() error {
	// Read stdin with short timeout for hook JSON (Claude passes {"session_id": "..."})
	claudeSessionID := readClaudeSessionID()

	// Write pane→agent mapping if we got a session ID
	if claudeSessionID != "" {
		writePaneAgentMapping(claudeSessionID)
	}

	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	if !cfg.Memory.Enabled || !cfg.Memory.ContextInjection {
		return nil
	}

	s, err := session.FindActiveSession()
	if err != nil {
		return nil
	}

	// If BAY_PRIOR_AGENT is set, this is a cold boot fallback —
	// filter context to only this agent's prior summaries
	priorAgent := os.Getenv("BAY_PRIOR_AGENT")

	ctx, err := memory.RenderContextForAgent(s.Name, priorAgent)
	if err != nil {
		return nil
	}

	if ctx != "" {
		fmt.Print(ctx)
	}
	return nil
}

// readClaudeSessionID reads JSON from stdin with a short timeout and extracts session_id.
func readClaudeSessionID() string {
	// Set a short read deadline — don't block if no input
	done := make(chan []byte, 1)
	go func() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			done <- nil
			return
		}
		done <- data
	}()

	var data []byte
	select {
	case data = <-done:
	case <-time.After(500 * time.Millisecond):
		return ""
	}

	if len(data) == 0 {
		return ""
	}

	var hookData struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &hookData); err != nil {
		return ""
	}
	return hookData.SessionID
}

// writePaneAgentMapping writes a mapping file at ~/.bay/pane-agents/{pane_id}
// containing the claude session ID.
func writePaneAgentMapping(claudeSessionID string) {
	// Get tmux pane ID
	cmd := exec.Command("tmux", "display-message", "-p", "#{pane_id}")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	paneID := strings.TrimSpace(string(out))
	if paneID == "" {
		return
	}

	dir := config.PaneAgentsDir()
	os.MkdirAll(dir, 0755)

	// Sanitize pane ID for filename (e.g., %5 → %5)
	filename := filepath.Join(dir, paneID)
	os.WriteFile(filename, []byte(claudeSessionID), 0644)
}
