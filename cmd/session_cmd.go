package cmd

import (
	"fmt"
	"os"
	"time"

	"bay/internal/hooks"
	"bay/internal/memory"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
	"bay/internal/worktree"
)

// SessionCmd handles the `bay session` subcommands.
func SessionCmd(args []string) error {
	if len(args) == 0 {
		printSessionHelp()
		return nil
	}

	switch args[0] {
	case "ls", "list":
		showArchived := len(args) > 1 && (args[1] == "--archived" || args[1] == "-a")
		return sessionLs(showArchived)

	case "kill":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay session kill <name>")
			return nil
		}
		return sessionKill(args[1])

	case "archive":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay session archive <name>")
			return nil
		}
		return sessionArchive(args[1])

	case "unarchive":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay session unarchive <name>")
			return nil
		}
		return sessionUnarchive(args[1])

	case "show":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return sessionShow(sessionName)

	case "help", "--help", "-h":
		printSessionHelp()
		return nil

	default:
		fmt.Fprintf(os.Stderr, "Unknown session command: %s\n", args[0])
		printSessionHelp()
		return nil
	}
}

func sessionLs(showArchived bool) error {
	if showArchived {
		archived, err := session.ListArchived()
		if err != nil {
			return fmt.Errorf("listing archived sessions: %w", err)
		}
		if len(archived) == 0 {
			fmt.Println("No archived sessions.")
			return nil
		}
		fmt.Println("archived sessions:")
		fmt.Println()
		for _, s := range archived {
			days := int(time.Since(s.ArchivedAt).Hours() / 24)
			fmt.Printf("  %-25s  repo: %-15s  (archived %dd ago)\n", s.Name, s.Repo, days)
		}
		fmt.Println()
		fmt.Printf("  %d archived session(s)\n", len(archived))
		return nil
	}

	sessions, err := session.List()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No bay sessions.")
	} else {
		fmt.Println("bay sessions:")
		fmt.Println()
		for _, s := range sessions {
			wt := ""
			if s.IsWorktree {
				wt = fmt.Sprintf(" [worktree: %s]", s.WorktreeBranch)
			}
			fmt.Printf("  %-25s  repo: %-15s%s\n", s.Name, s.Repo, wt)
		}
		fmt.Println()
		fmt.Printf("  %d session(s)\n", len(sessions))
	}

	archived, _ := session.ListArchived()
	if len(archived) > 0 {
		fmt.Printf("  %d archived (bay session ls --archived)\n", len(archived))
	}

	return nil
}

func sessionArchive(name string) error {
	s, err := session.Load(name)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", name, err)
	}
	if s.IsArchived() {
		fmt.Printf("Session '%s' is already archived\n", name)
		return nil
	}
	if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
		baytmux.KillWindow(s.TmuxWindow)
	}
	if err := session.Archive(name); err != nil {
		return fmt.Errorf("archiving session: %w", err)
	}
	fmt.Printf("Archived session '%s'\n", name)
	return nil
}

func sessionUnarchive(name string) error {
	s, err := session.Load(name)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", name, err)
	}
	if !s.IsArchived() {
		fmt.Printf("Session '%s' is not archived\n", name)
		return nil
	}
	if err := session.Unarchive(name); err != nil {
		return fmt.Errorf("unarchiving session: %w", err)
	}
	fmt.Printf("Unarchived session '%s'\n", name)
	return nil
}

func sessionKill(name string) error {
	s, err := session.Load(name)
	if err != nil {
		return fmt.Errorf("session '%s' not found: %w", name, err)
	}

	if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
		baytmux.KillWindow(s.TmuxWindow)
	}

	if s.IsWorktree && s.WorktreeBranch != "" {
		if err := worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch); err != nil {
			fmt.Printf("Warning: worktree cleanup failed: %v\n", err)
		}
	}

	hooks.OnSessionDelete(name)

	if err := session.Delete(name); err != nil {
		return fmt.Errorf("deleting session file: %w", err)
	}

	fmt.Printf("Killed session '%s'\n", name)
	return nil
}

func sessionShow(sessionName string) error {
	var err error
	sessionName, err = resolveSessionName(sessionName)
	if err != nil {
		return err
	}

	s, err := session.Load(sessionName)
	if err != nil {
		return fmt.Errorf("loading session: %w", err)
	}

	fmt.Printf("Session:  %s\n", s.Name)
	fmt.Printf("Repo:     %s\n", s.Repo)
	if s.WorktreeBranch != "" {
		fmt.Printf("Branch:   %s\n", s.WorktreeBranch)
	}
	if s.Purpose != "" {
		fmt.Printf("\nPurpose:\n  %s\n", s.Purpose)
	}

	tasks, _ := memory.ListTasks(s.Name)
	if len(tasks) > 0 {
		fmt.Printf("\nChecklist:\n")
		for i, t := range tasks {
			marker := "[ ]"
			if t.Status == "done" {
				marker = "[x]"
			}
			fmt.Printf("  %s %d. %s\n", marker, i+1, t.Title)
		}
	}

	return nil
}

func printSessionHelp() {
	fmt.Println(`bay session — Session lifecycle

Usage:
  bay session ls                     List all sessions.
  bay session ls --archived          List archived sessions.
  bay session kill <name>            Kill a session and clean up resources.
  bay session archive <name>         Archive a session.
  bay session unarchive <name>       Restore an archived session.
  bay session show [session]         Show session info (purpose, checklist, repo).`)
}
