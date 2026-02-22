package main

import (
	"fmt"
	"os"

	"github.com/anawarkar/bay/cmd"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		if err := cmd.Root(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
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
		"  bay              Launch bay (opens sidebar in tmux)\n" +
		"  bay setup        Run the setup wizard\n" +
		"  bay ls           List all sessions\n" +
		"  bay kill <name>  Kill a session\n" +
		"  bay keybinds     Show keybind reference and terminal setup tips\n" +
		"  bay build        Rebuild bay from latest source\n" +
		"  bay help         Show this help\n" +
		"\n" +
		"Sidebar keybindings:\n" +
		"  n       New session\n" +
		"  d       Delete session\n" +
		"  r       Rename session\n" +
		"  Enter   Switch to session\n" +
		"  c       Add Claude Code pane\n" +
		"  Tab     Expand/collapse repo\n" +
		"  j/k     Navigate\n" +
		"  s       Re-run setup\n" +
		"  q       Quit\n" +
		"\n" +
		"Pane management (` then):\n" +
		"  Arrow           Navigate panes\n" +
		"  D               Vertical split\n" +
		"  Shift+D         Horizontal split\n" +
		"  W               Close pane\n" +
		"  S               Toggle sidebar/dev focus\n" +
		"  ``              Type a literal backtick\n" +
		"  Click           Focus any pane"
	fmt.Println(help)
}
