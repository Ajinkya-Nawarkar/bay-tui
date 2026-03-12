package cmd

import (
	"fmt"
	"os"

	"bay/internal/config"
	"bay/internal/memory"
	"bay/internal/session"
)

// Context outputs session context to stdout for use as a Claude SessionStart hook.
// This must NEVER return an error — a non-zero exit causes Claude to show "startup hook error".
func Context() error {
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

	// Look up pane's assigned TaskID from session YAML
	paneTaskID := 0
	paneID := os.Getenv("TMUX_PANE")
	if paneID != "" {
		for _, p := range s.Panes {
			if p.PaneID == paneID {
				paneTaskID = p.TaskID
				break
			}
		}
	}

	ctx, err := memory.RenderContextDB(nil, s.Name, s.Note, paneTaskID)
	if err != nil {
		return nil
	}

	if ctx != "" {
		fmt.Print(ctx)
	}
	return nil
}
