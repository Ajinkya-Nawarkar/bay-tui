package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"bay/internal/config"
	"bay/internal/constants"
	"bay/internal/memory"
)

// Ctx handles the `bay ctx` subcommands — search and config.
func Ctx(args []string) error {
	if len(args) == 0 {
		return Context()
	}

	switch args[0] {
	case "search":
		return ctxSearch(args[1:])

	case "config":
		return ctxConfig(args[1:])

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
		ts := e.Timestamp.Format(constants.TimeFmtCompact)
		content := truncatePreview(e.Content)
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

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "on", "true", "yes", "1", "enabled":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func printCtxHelp() {
	fmt.Println(`bay ctx — Search and configuration

Usage:

  Search
    bay ctx search "query" [--session S]
                                       Full-text search across all session history.
                                       Finds past terminal output, notes, and summaries.

  Configuration
    bay ctx config                     Show current memory feature settings.
    bay ctx config <feature> on|off    Toggle: enabled, episodic_logging, auto_summarize,
                                       context_injection. Also: context_budget <int>.

Context file management has moved to bctx:
    bctx files                         List registered context files.
    bctx add <name> <path>             Register a context file.
    bctx rm <name>                     Remove a context file.
    bctx toggle <name>                 Enable/disable a context file.`)
}
