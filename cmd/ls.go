package cmd

import (
	"fmt"

	"bay/internal/session"
)

// Ls lists all bay sessions.
func Ls() error {
	sessions, err := session.List()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No bay sessions.")
		return nil
	}

	fmt.Println("bay sessions:")
	fmt.Println()
	for _, s := range sessions {
		wt := ""
		if s.IsWorktree {
			wt = fmt.Sprintf(" [worktree: %s]", s.WorktreeBranch)
		}
		fmt.Printf("  %-25s  repo: %-15s%s\n", s.Name, s.Repo, wt)
	}
	fmt.Println()
	fmt.Printf("  %d session(s)\n", len(sessions))

	return nil
}
