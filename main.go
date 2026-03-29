package main

import (
	"fmt"
	"os"

	"bay/cmd"
	"bay/internal/logging"
)

var Version = "dev"

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error("PANIC: %v", r)
			fmt.Fprintf(os.Stderr, "bay crashed: %v\n", r)
			exitCode = 1
		}
		logging.Close()
	}()

	args := os.Args[1:]

	if len(args) == 0 {
		if err := cmd.Root(false); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	switch args[0] {
	case "-f", "--fresh":
		if err := cmd.Root(true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "--tui":
		// Internal flag: run TUI directly (called from within tmux)
		if err := cmd.RunTUIDirectly(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "setup":
		if err := cmd.Setup(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "ctx":
		if err := cmd.Ctx(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "internal":
		if err := cmd.Internal(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "session":
		if err := cmd.SessionCmd(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "task":
		if err := cmd.Task(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "agent":
		if err := cmd.Agent(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "refresh":
		if err := cmd.Refresh(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "keybinds":
		if err := cmd.Keybinds(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "build":
		if err := cmd.Build(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "upgrade":
		if err := cmd.Upgrade(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "beta":
		if err := cmd.Beta(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "uninstall":
		if err := cmd.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "search":
		if err := cmd.Internal([]string{"search"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "purpose":
		if err := cmd.Internal([]string{"purpose"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "sync-panes":
		// Legacy alias — routes to bay internal sync-panes
		if err := cmd.Internal([]string{"sync-panes"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}

	case "version", "--version", "-v":
		fmt.Printf("bay %s — by Ajinkya Nawarkar\n", Version)
		fmt.Println("https://github.com/Ajinkya-Nawarkar/bay-tui")

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printHelp()
		return 1
	}

	return 0
}

func printHelp() {
	help := `bay — Developer Session & Agent Hub

Usage:
  bay              Launch bay (opens topbar in tmux)
  bay -f           Fresh start (kill existing session, relaunch)

Sessions:
  bay session ls             List all sessions
  bay session kill <name>    Kill a session and clean up its resources
  bay session show [name]    Show session info (purpose, checklist, repo)

Session Purpose:
  bay task "purpose text"    Set session purpose
  bay task add "item"        Add checklist item
  bay task ls                Show purpose + checklist
  bay task done <id>         Mark item done
  bay task rm <id>           Remove item
  bay task clear             Clear all checklist items

Views:
  bay search                 Search sessions + activity dashboard
  bay purpose                Purpose and checklist editor

Config:
  bay ctx config             Show/toggle context injection settings

Infrastructure:
  bay setup        Run the setup wizard
  bay refresh      Re-sync panes and restart topbar TUI
  bay keybinds     Show keybind reference and terminal setup tips
  bay build        Rebuild bay from latest source
  bay upgrade      Download and install latest release
  bay beta         Install latest beta (pre-release) version
  bay uninstall    Remove all bay data and Claude hooks
  bay version      Show version
  bay help         Show this help

Top bar (` + "`" + ` prefix):
  ` + "`" + `+Tab   Cycle session    ` + "`" + `+1-9   Jump to session
  ` + "`" + `+space  Enter bay view mode

Bay view mode (` + "`" + `+space to enter, esc to leave):
  h/l     Switch repo      n/d/R   New/delete/rename session
  N       Edit purpose     /       Search sessions
  s       Status dashboard Enter   Activate session
  q       Quit bay         esc     Leave bay view

Pane management (` + "`" + ` then):
  Arrow           Navigate panes
  d               Vertical split
  D               Horizontal split
  w               Close pane
  {/}             Swap pane up/down
  ` + "``" + `              Type a literal backtick
  Click           Focus any pane`
	fmt.Println(help)
}
