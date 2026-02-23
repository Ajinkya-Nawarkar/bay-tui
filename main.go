package main

import (
	"fmt"
	"os"

	"bay/cmd"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		if err := cmd.Root(false); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "-f", "--fresh":
		if err := cmd.Root(true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "--tui":
		// Internal flag: run TUI directly (called from within tmux)
		if err := cmd.RunTUIDirectly(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "setup":
		if err := cmd.Setup(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "ls", "list":
		if err := cmd.Ls(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "kill":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay kill <session-name>")
			os.Exit(1)
		}
		if err := cmd.Kill(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "keybinds":
		if err := cmd.Keybinds(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "build":
		if err := cmd.Build(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "uninstall":
		if err := cmd.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "mem":
		if err := cmd.Mem(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "search":
		if err := cmd.Search(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "context":
		if err := cmd.Context(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "rules":
		if err := cmd.Rules(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	help := "bay — Developer Session & Agent Hub\n" +
		"\n" +
		"Usage:\n" +
		"  bay              Launch bay (opens top bar in tmux)\n" +
		"  bay -f           Fresh start (kill existing session, relaunch)\n" +
		"  bay setup        Run the setup wizard\n" +
		"  bay ls           List all sessions\n" +
		"  bay kill <name>  Kill a session\n" +
		"  bay keybinds     Show keybind reference and terminal setup tips\n" +
		"  bay build        Rebuild bay from latest source\n" +
		"  bay uninstall    Remove all bay data and Claude hooks\n" +
		"  bay help         Show this help\n" +
		"\n" +
		"Memory:\n" +
		"  bay mem show [session]   Show working memory state\n" +
		"  bay mem task \"desc\"      Set current task\n" +
		"  bay mem note \"text\"      Add note to episodic log\n" +
		"  bay mem log [-n 50]      Show episodic log\n" +
		"  bay mem clear [session]  Clear session memory\n" +
		"  bay mem config           Show/toggle memory features\n" +
		"  bay search \"query\"       Full-text search across all sessions\n" +
		"  bay context              Output session context (Claude hook)\n" +
		"  bay rules ls|add|rm|toggle  Manage context injection rules\n" +
		"\n" +
		"Top bar (` prefix):\n" +
		"  `+Tab   Cycle session    `+0-9   Jump to session\n" +
		"  `+r     Cycle repo       `+q     Toggle focus mode\n" +
		"\n" +
		"Focused mode (`+q to enter, esc to leave):\n" +
		"  h/l     Switch repo      n/d/R   New/delete/rename session\n" +
		"  m       Memory viewer    Enter   Activate session\n" +
		"\n" +
		"Pane management (` then):\n" +
		"  Arrow           Navigate panes\n" +
		"  d               Vertical split\n" +
		"  D               Horizontal split\n" +
		"  w               Close pane\n" +
		"  s               Toggle topbar/dev focus\n" +
		"  ``              Type a literal backtick\n" +
		"  Click           Focus any pane"
	fmt.Println(help)
}
