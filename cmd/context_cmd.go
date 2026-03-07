package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	bayctx "bay/internal/context"
	"bay/internal/session"
)

// ContextCmd handles the `bay context` subcommands.
// With no args, outputs session context (used by Claude SessionStart hook).
func ContextCmd(args []string) error {
	if len(args) == 0 {
		return Context()
	}

	switch args[0] {
	case "ls", "list":
		return contextLs()
	case "add":
		return contextAdd(args[1:])
	case "rm", "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay context rm <name>")
			return nil
		}
		return contextRm(args[1])
	case "toggle":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay context toggle <name>")
			return nil
		}
		return contextToggle(args[1])
	case "sync":
		return contextSync()
	default:
		fmt.Fprintf(os.Stderr, "Unknown context command: %s\n", args[0])
		printContextHelp()
		return nil
	}
}

func contextLs() error {
	list, err := bayctx.List()
	if err != nil {
		return fmt.Errorf("listing context files: %w", err)
	}

	if len(list) == 0 {
		fmt.Println("No context files registered.")
		return nil
	}

	fmt.Printf("%-20s %-10s %-8s %-15s %s\n", "NAME", "CATEGORY", "STATUS", "SCOPE", "PATH")
	for _, f := range list {
		status := "on"
		if !f.Enabled {
			status = "off"
		}
		cat := f.Category
		if cat == "" {
			cat = "rules"
		}
		fmt.Printf("%-20s %-10s %-8s %-15s %s\n", f.Name, cat, status, f.Scope, f.Path)
	}
	return nil
}

func contextAdd(args []string) error {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bay context add <name> <path> [--scope S] [--category C]")
		return nil
	}

	name := args[0]
	path := args[1]
	scope := "global"
	category := "rules"

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", path)
	}

	for i := 2; i < len(args); i++ {
		if args[i] == "--scope" && i+1 < len(args) {
			scope = args[i+1]
			i++
		}
		if args[i] == "--category" && i+1 < len(args) {
			category = args[i+1]
			i++
		}
	}

	if err := bayctx.Add(name, path, scope, category); err != nil {
		return fmt.Errorf("adding context file: %w", err)
	}

	fmt.Printf("Added '%s' (%s, %s) → %s\n", name, category, scope, path)
	syncAllWorktreeSessions()
	return nil
}

func contextRm(name string) error {
	if err := bayctx.Remove(name); err != nil {
		return fmt.Errorf("removing context file: %w", err)
	}
	fmt.Printf("Removed '%s'\n", name)
	syncAllWorktreeSessions()
	return nil
}

func contextToggle(name string) error {
	if err := bayctx.Toggle(name); err != nil {
		return fmt.Errorf("toggling context file: %w", err)
	}

	// Show new state
	list, _ := bayctx.List()
	for _, f := range list {
		if f.Name == name {
			state := "enabled"
			if !f.Enabled {
				state = "disabled"
			}
			fmt.Printf("'%s' is now %s\n", name, state)
			break
		}
	}

	syncAllWorktreeSessions()
	return nil
}

func contextSync() error {
	fmt.Println("Syncing context files to all worktree sessions...")
	syncAllWorktreeSessions()
	fmt.Println("Done.")
	return nil
}

// syncAllWorktreeSessions re-syncs context files to all active worktree sessions.
func syncAllWorktreeSessions() {
	sessions, err := session.List()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.IsWorktree {
			bayctx.SyncRulesToWorktree(s.WorkingDir, s.Repo)
		}
	}
}

func printContextHelp() {
	fmt.Println(`bay context — Context file management

Usage:
  bay context ls                                     List all registered context files
  bay context add <name> <path> [--scope S] [--category C]  Register a context file
  bay context rm <name>                              Remove a context file
  bay context toggle <name>                          Enable/disable a context file
  bay context sync                                   Re-sync all worktree sessions

Scope formats:
  global        Applies to all sessions (default)
  repo:<name>   Applies only to sessions for that repo

Categories:
  rules         Rules and guidelines (default)
  docs          Documentation and references
  standards     Coding standards`)
}
