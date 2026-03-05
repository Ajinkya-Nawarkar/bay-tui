package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"bay/internal/rules"
	"bay/internal/session"
)

// Rules handles the `bay rules` subcommands.
func Rules(args []string) error {
	if len(args) == 0 {
		printRulesHelp()
		return nil
	}

	switch args[0] {
	case "ls", "list":
		return rulesLs()
	case "add":
		return rulesAdd(args[1:])
	case "rm", "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay rules rm <name>")
			return nil
		}
		return rulesRm(args[1])
	case "toggle":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay rules toggle <name>")
			return nil
		}
		return rulesToggle(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown rules command: %s\n", args[0])
		printRulesHelp()
		return nil
	}
}

func rulesLs() error {
	list, err := rules.List()
	if err != nil {
		return fmt.Errorf("listing rules: %w", err)
	}

	if len(list) == 0 {
		fmt.Println("No rules registered.")
		return nil
	}

	fmt.Printf("%-20s %-8s %-15s %s\n", "NAME", "STATUS", "SCOPE", "PATH")
	for _, r := range list {
		status := "on"
		if !r.Enabled {
			status = "off"
		}
		fmt.Printf("%-20s %-8s %-15s %s\n", r.Name, status, r.Scope, r.Path)
	}
	return nil
}

func rulesAdd(args []string) error {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bay rules add <name> <path> [--scope repo:name]")
		return nil
	}

	name := args[0]
	path := args[1]
	scope := "global"

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
	}

	if err := rules.Add(name, path, scope); err != nil {
		return fmt.Errorf("adding rule: %w", err)
	}

	fmt.Printf("Added rule '%s' (%s) → %s\n", name, scope, path)
	syncAllWorktreeSessions()
	return nil
}

func rulesRm(name string) error {
	if err := rules.Remove(name); err != nil {
		return fmt.Errorf("removing rule: %w", err)
	}
	fmt.Printf("Removed rule '%s'\n", name)
	syncAllWorktreeSessions()
	return nil
}

func rulesToggle(name string) error {
	if err := rules.Toggle(name); err != nil {
		return fmt.Errorf("toggling rule: %w", err)
	}

	// Show new state
	list, _ := rules.List()
	for _, r := range list {
		if r.Name == name {
			state := "enabled"
			if !r.Enabled {
				state = "disabled"
			}
			fmt.Printf("Rule '%s' is now %s\n", name, state)
			syncAllWorktreeSessions()
			return nil
		}
	}

	fmt.Printf("Toggled rule '%s'\n", name)
	syncAllWorktreeSessions()
	return nil
}

// syncAllWorktreeSessions re-syncs rules to all active worktree sessions.
func syncAllWorktreeSessions() {
	sessions, err := session.List()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.IsWorktree {
			rules.SyncRulesToWorktree(s.WorkingDir, s.Repo)
		}
	}
}

func printRulesHelp() {
	fmt.Println(`bay rules — Context injection rule management

Usage:
  bay rules ls                              List all registered rules
  bay rules add <name> <path> [--scope S]   Register a context file
  bay rules rm <name>                       Remove a rule
  bay rules toggle <name>                   Enable/disable a rule

Scope formats:
  global        Rule applies to all sessions (default)
  repo:<name>   Rule applies only to sessions for that repo`)
}
