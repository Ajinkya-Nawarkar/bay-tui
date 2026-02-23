package cmd

import (
	"fmt"

	"bay/internal/config"
	"bay/internal/memory"
	"bay/internal/session"
)

// Context outputs session context to stdout for use as a Claude SessionStart hook.
func Context() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if !cfg.Memory.Enabled || !cfg.Memory.ContextInjection {
		return nil
	}

	// Detect active session from tmux window
	s, err := session.FindActiveSession()
	if err != nil {
		// Not in a bay session — silently exit (hook should be non-disruptive)
		return nil
	}

	ctx, err := memory.RenderContext(s.Name)
	if err != nil {
		return fmt.Errorf("rendering context: %w", err)
	}

	if ctx != "" {
		fmt.Print(ctx)
	}
	return nil
}
