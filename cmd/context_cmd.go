package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	bayctx "bay/internal/context"
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
	case "cleanup":
		return contextCleanup()
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

	fmt.Printf("%-20s %-10s %-8s %-8s %-15s %-30s %s\n", "NAME", "TYPE", "CATEGORY", "STATUS", "SCOPE", "DESCRIPTION", "PATH")
	for _, f := range list {
		status := "on"
		if !f.Enabled {
			status = "off"
		}
		cat := f.Category
		if cat == "" {
			cat = "rules"
		}
		typ := f.Type
		if typ == "" {
			typ = "rules"
		}
		desc := f.Description
		if len(desc) > 28 {
			desc = desc[:28] + ".."
		}
		fmt.Printf("%-20s %-10s %-8s %-8s %-15s %-30s %s\n", f.Name, typ, cat, status, f.Scope, desc, f.Path)
	}
	return nil
}

func contextAdd(args []string) error {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bay context add <name> <path> [--scope S] [--category C] [--type T] [--description D]")
		return nil
	}

	name := args[0]
	path := args[1]
	scope := "global"
	category := "rules"
	typ := "rules"
	description := ""

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
		if args[i] == "--type" && i+1 < len(args) {
			typ = args[i+1]
			i++
		}
		if args[i] == "--description" && i+1 < len(args) {
			description = args[i+1]
			i++
		}
	}

	if err := bayctx.Add(name, path, scope, category, typ, description); err != nil {
		return fmt.Errorf("adding context file: %w", err)
	}

	fmt.Printf("Added '%s' (%s/%s, %s) → %s\n", name, typ, category, scope, path)
	regenerateNavigator()
	return nil
}

func contextRm(name string) error {
	if err := bayctx.Remove(name); err != nil {
		return fmt.Errorf("removing context file: %w", err)
	}
	fmt.Printf("Removed '%s'\n", name)
	regenerateNavigator()
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

	regenerateNavigator()
	return nil
}

func contextSync() error {
	fmt.Println("Regenerating navigator and indexes...")
	regenerateNavigator()
	fmt.Println("Done.")
	return nil
}

func contextCleanup() error {
	fmt.Println("Removing old worktree-synced .claude/rules/bay/ directories...")
	cleaned, err := bayctx.CleanupWorktreeRules()
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	if cleaned == 0 {
		fmt.Println("No worktree rule directories found to clean up.")
	} else {
		fmt.Printf("Cleaned %d worktree(s).\n", cleaned)
	}
	return nil
}

// regenerateNavigator regenerates ~/.bay/CLAUDE.md and all index.yaml files.
func regenerateNavigator() {
	bayctx.EnsureResourceDirs()
	bayctx.GenerateNavigator()
	bayctx.GenerateAllIndexes()
}

func printContextHelp() {
	fmt.Println(`bay context — Context file management

Usage:
  bay context ls                                     List all registered context files
  bay context add <name> <path> [flags]              Register a context file
  bay context rm <name>                              Remove a context file
  bay context toggle <name>                          Enable/disable a context file
  bay context sync                                   Regenerate navigator and indexes
  bay context cleanup                                Remove old worktree-synced files

Flags for add:
  --scope S          global (default) or repo:<name>
  --category C       rules, docs, standards (default: rules)
  --type T           rules, skills, agents, plugins (default: rules)
  --description D    Short description for the resource catalog`)
}
