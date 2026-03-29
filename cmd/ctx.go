package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"bay/internal/config"
)

// Ctx handles the `bay ctx` subcommands.
func Ctx(args []string) error {
	if len(args) == 0 {
		return Context()
	}

	switch args[0] {
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

func ctxConfig(args []string) error {
	if len(args) == 0 {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		m := cfg.Memory
		fmt.Printf("Context Configuration:\n")
		fmt.Printf("  enabled:            %v\n", m.Enabled)
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

func printCtxHelp() {
	fmt.Println(`bay ctx — Context injection configuration

Usage:
  bay ctx                            Output session context (used by hooks).
  bay ctx config                     Show context injection settings.
  bay ctx config <feature> on|off    Toggle: enabled, context_injection.
                                     Also: context_budget <int>.`)
}
