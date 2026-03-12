package cmd

import (
	"fmt"
	"os"

	"bay/internal/hooks"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
	"bay/internal/worktree"
)

// SessionCmd handles the `bay session` subcommands — session lifecycle.
func SessionCmd(args []string) error {
	if len(args) == 0 {
		printSessionHelp()
		return nil
	}

	switch args[0] {
	case "ls", "list":
		return sessionLs()

	case "kill":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay session kill <name>")
			return nil
		}
		return sessionKill(args[1])

	case "help", "--help", "-h":
		printSessionHelp()
		return nil

	default:
		fmt.Fprintf(os.Stderr, "Unknown session command: %s\n", args[0])
		printSessionHelp()
		return nil
	}
}

func sessionLs() error {
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

func sessionKill(name string) error {
	s, err := session.Load(name)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", name, err)
	}

	if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
		baytmux.KillWindow(s.TmuxWindow)
	}

	if s.IsWorktree && s.WorktreeBranch != "" {
		if err := worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch); err != nil {
			fmt.Printf("Warning: worktree cleanup failed: %v\n", err)
		}
	}

	hooks.OnSessionDelete(name)

	if err := session.Delete(name); err != nil {
		return fmt.Errorf("deleting session file: %w", err)
	}

	fmt.Printf("Killed session '%s'\n", name)
	return nil
}

func printSessionHelp() {
	fmt.Println(`bay session — Session lifecycle management

List, inspect, and destroy bay sessions. Each session is a tmux window
with its own panes, worktree, and memory state.

Usage:
  bay session ls             List all sessions with repo and worktree info.
  bay session kill <name>    Kill a session: destroys the tmux window, removes
                             the worktree, cleans up memory, and deletes the
                             session file. Cannot be undone.`)
}
