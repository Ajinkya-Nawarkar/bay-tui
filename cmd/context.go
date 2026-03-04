package cmd

import (
	"fmt"

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

	ctx, err := memory.RenderContext(s.Name)
	if err != nil {
		return nil
	}

	if ctx != "" {
		fmt.Print(ctx)
	}
	return nil
}
