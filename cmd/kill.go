package cmd

import (
	"fmt"

	"github.com/anawarkar/bay/internal/session"
	baytmux "github.com/anawarkar/bay/internal/tmux"
	"github.com/anawarkar/bay/internal/worktree"
)

// Kill destroys a bay session by name (removes YAML + worktree).
func Kill(name string) error {
	s, err := session.Load(name)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", name, err)
	}

	// Kill the session's tmux window if it exists
	if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
		baytmux.KillWindow(s.TmuxWindow)
	}

	// Remove worktree if applicable
	if s.IsWorktree && s.WorktreeBranch != "" {
		if err := worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch); err != nil {
			fmt.Printf("Warning: worktree cleanup failed: %v\n", err)
		}
	}

	// Delete session file
	if err := session.Delete(name); err != nil {
		return fmt.Errorf("deleting session file: %w", err)
	}

	fmt.Printf("Killed session '%s'\n", name)
	return nil
}
