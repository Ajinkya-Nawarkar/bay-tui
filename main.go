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

	case "ctx", "context", "rules":
		if err := cmd.Ctx(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "session":
		if err := cmd.SessionCmd(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "task":
		if err := cmd.Task(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "agent":
		if err := cmd.Agent(args[1:]); err != nil {
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

	case "upgrade":
		if err := cmd.Upgrade(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "uninstall":
		if err := cmd.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "sync-panes":
		if err := cmd.SyncPanes(); err != nil {
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
	help := `bay — Developer Session & Agent Hub

Usage:
  bay              Launch bay (opens topbar in tmux)
  bay -f           Fresh start (kill existing session, relaunch)

Sessions:
  bay session ls             List all sessions
  bay session kill <name>    Kill a session and clean up its resources

Tasks:
  bay task "description"     Create a task in the current session
  bay task add "desc" [-p N] Add a subtask (optionally under task #N)
  bay task ls                List all tasks with status
  bay task done <id>         Mark task done
  bay task doing <id>        Mark task in-progress
  bay task rm <id>           Remove a task (and subtasks)
  bay task assign <id>       Assign the current pane to a task
  bay task clear             Clear all tasks for the session

Context & Memory:
  bay ctx show [session]     Show working state (tasks, summary, repo, branch)
  bay ctx note "text"        Add note to session history
  bay ctx history [-n 50]    Show episodic log
  bay ctx search "query"     Full-text search across all sessions
  bay ctx files              List registered context files
  bay ctx add <name> <path>  Register a context file for agent injection
  bay ctx rm <name>          Remove a context file
  bay ctx toggle <name>      Enable/disable a context file
  bay ctx config             Show/toggle memory features
  bay ctx clear [session]    Clear all memory for a session

Infrastructure:
  bay setup        Run the setup wizard
  bay keybinds     Show keybind reference and terminal setup tips
  bay build        Rebuild bay from latest source
  bay upgrade      Download and install latest release
  bay uninstall    Remove all bay data and Claude hooks
  bay help         Show this help

Top bar (` + "`" + ` prefix):
  ` + "`" + `+Tab   Cycle session    ` + "`" + `+1-9   Jump to session
  ` + "`" + `+r     Cycle repo       ` + "`" + `+space  Toggle focus mode

Focused mode (` + "`" + `+space to enter, esc to leave):
  h/l     Switch repo      n/d/R   New/delete/rename session
  N       Edit session note
  m       Memory viewer    Enter   Activate session
  q       Quit bay         esc     Leave focus mode

Pane management (` + "`" + ` then):
  Arrow           Navigate panes
  d               Vertical split
  D               Horizontal split
  w               Close pane
  {/}             Swap pane up/down
  s               Toggle topbar/dev focus
  ` + "``" + `              Type a literal backtick
  Click           Focus any pane`
	fmt.Println(help)
}
