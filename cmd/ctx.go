package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	bayctx "bay/internal/context"
	"bay/internal/config"
	"bay/internal/memory"
)

// Ctx handles the `bay ctx` subcommands — context files, search, and config.
func Ctx(args []string) error {
	if len(args) == 0 {
		return Context()
	}

	switch args[0] {
	// --- Search ---
	case "search":
		return ctxSearch(args[1:])

	// --- Config ---
	case "config":
		return ctxConfig(args[1:])

	// --- Context files ---
	case "files":
		return ctxFiles()

	case "add":
		return ctxAdd(args[1:])

	case "rm", "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay ctx rm <name>")
			return nil
		}
		return ctxRm(args[1])

	case "toggle":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay ctx toggle <name>")
			return nil
		}
		return ctxToggle(args[1])

	case "sync":
		return ctxSync()

	case "cleanup":
		return ctxCleanup()

	case "help", "--help", "-h":
		printCtxHelp()
		return nil

	default:
		fmt.Fprintf(os.Stderr, "Unknown ctx command: %s\n", args[0])
		printCtxHelp()
		return nil
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func ctxSearch(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: bay ctx search \"query\" [--session name]")
		return nil
	}

	query := args[0]
	sessionFilter := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "--session" && i+1 < len(args) {
			sessionFilter = args[i+1]
			i++
		}
	}

	results, err := memory.SearchEpisodic(query, sessionFilter)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No results for '%s'\n", query)
		return nil
	}

	fmt.Printf("Search results for '%s' (%d matches):\n\n", query, len(results))
	for _, e := range results {
		ts := e.Timestamp.Format("2006-01-02 15:04")
		content := e.Content
		if len(content) > 120 {
			content = content[:117] + "..."
		}
		fmt.Printf("  [%s] %-12s %-15s %s\n", ts, e.SessionID, e.Type, content)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

func ctxConfig(args []string) error {
	if len(args) == 0 {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		m := cfg.Memory
		fmt.Printf("Memory Configuration:\n")
		fmt.Printf("  enabled:            %v\n", m.Enabled)
		fmt.Printf("  episodic_logging:   %v\n", m.EpisodicLogging)
		fmt.Printf("  auto_summarize:     %v\n", m.AutoSummarize)
		fmt.Printf("  context_injection:  %v\n", m.ContextInjection)
		fmt.Printf("  context_budget:     %d\n", m.ContextBudget)
		return nil
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bay ctx config <feature> on|off|<value>")
		return nil
	}

	feature := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch feature {
	case "enabled":
		cfg.Memory.Enabled = parseBool(args[1])
	case "episodic_logging":
		cfg.Memory.EpisodicLogging = parseBool(args[1])
	case "auto_summarize":
		cfg.Memory.AutoSummarize = parseBool(args[1])
	case "context_injection":
		cfg.Memory.ContextInjection = parseBool(args[1])
	case "context_budget":
		v, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("context_budget must be an integer: %w", err)
		}
		cfg.Memory.ContextBudget = v
	default:
		return fmt.Errorf("unknown feature: %s", feature)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("%s set\n", feature)
	return nil
}

// ---------------------------------------------------------------------------
// Context file handlers
// ---------------------------------------------------------------------------

func ctxFiles() error {
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
			desc = desc[:25] + "..."
		}
		fmt.Printf("%-20s %-10s %-8s %-8s %-15s %-30s %s\n", f.Name, typ, cat, status, f.Scope, desc, f.Path)
	}
	return nil
}

func ctxAdd(args []string) error {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bay ctx add <name> <path> [--scope S] [--category C] [--type T] [--desc D]")
		return nil
	}

	name := args[0]
	path := args[1]
	scope := "global"
	category := "rules"
	typ := "rules"
	description := ""

	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}

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
		if args[i] == "--desc" && i+1 < len(args) {
			description = args[i+1]
			i++
		}
	}

	if err := bayctx.Add(name, path, scope, category, typ, description); err != nil {
		return fmt.Errorf("adding context file: %w", err)
	}

	fmt.Printf("Added '%s' (%s, %s) → %s\n", name, category, scope, path)
	regenerateNavigator()
	return nil
}

func ctxRm(name string) error {
	if err := bayctx.Remove(name); err != nil {
		return fmt.Errorf("removing context file: %w", err)
	}
	fmt.Printf("Removed '%s'\n", name)
	regenerateNavigator()
	return nil
}

func ctxToggle(name string) error {
	if err := bayctx.Toggle(name); err != nil {
		return fmt.Errorf("toggling context file: %w", err)
	}

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

func ctxSync() error {
	fmt.Println("Regenerating resource navigator...")
	regenerateNavigator()
	fmt.Println("Done.")
	return nil
}

func ctxCleanup() error {
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

// regenerateNavigator refreshes the resource navigator after context file changes.
func regenerateNavigator() {
	bayctx.EnsureResourceDirs()
	bayctx.GenerateNavigator()
	bayctx.GenerateAllIndexes()
}

func parseBool(s string) bool {
	return s == "on" || s == "true"
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func printCtxHelp() {
	fmt.Println(`bay ctx — Context files, search, and configuration

Manage what gets injected into agent context: context files, cross-session
search, and memory configuration.

Usage:

  Search
    bay ctx search "query" [--session S]
                                       Full-text search across all session history.
                                       Finds past terminal output, notes, and summaries.

  Context Files
    bay ctx files                      List all registered context files and their status.
    bay ctx add <name> <path> [--scope S] [--category C]
                                       Register a file to be injected into agent context.
                                       Scope: "global" (default) or "repo:<name>".
                                       Category: "rules" (default), "docs", "standards".
    bay ctx rm <name>                  Remove a registered context file.
    bay ctx toggle <name>              Enable or disable a context file without removing it.
    bay ctx sync                       Regenerate resource navigator and indexes.

  Configuration
    bay ctx config                     Show current memory feature settings.
    bay ctx config <feature> on|off    Toggle: enabled, episodic_logging, auto_summarize,
                                       context_injection. Also: context_budget <int>.`)
}
