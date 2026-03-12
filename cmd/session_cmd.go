package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"bay/internal/hooks"
	"bay/internal/memory"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
	"bay/internal/worktree"
)

// SessionCmd handles the `bay session` subcommands — session lifecycle and state.
func SessionCmd(args []string) error {
	if len(args) == 0 {
		printSessionHelp()
		return nil
	}

	switch args[0] {
	case "ls", "list":
		return sessionLs()

	case "kill":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay session kill <name>")
			return nil
		}
		return sessionKill(args[1])

	case "note":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay session note \"text\"")
			return nil
		}
		return sessionNote(strings.Join(args[1:], " "))

	case "show":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return sessionShow(sessionName)

	case "history":
		return sessionHistory(args[1:])

	case "clear":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return sessionClear(sessionName)

	case "help", "--help", "-h":
		printSessionHelp()
		return nil

	default:
		fmt.Fprintf(os.Stderr, "Unknown session command: %s\n", args[0])
		printSessionHelp()
		return nil
	}
}

func sessionLs() error {
	sessions, err := session.List()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No bay sessions.")
		return nil
	}

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

func sessionNote(text string) error {
	s, err := session.FindActiveSession()
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}

	if err := memory.AppendEpisodic(s.Name, "note", text, ""); err != nil {
		return fmt.Errorf("adding note: %w", err)
	}

	fmt.Println("Note saved.")
	return nil
}

func sessionShow(sessionName string) error {
	if sessionName == "" {
		s, err := session.FindActiveSession()
		if err != nil {
			return fmt.Errorf("no active session (specify one): %w", err)
		}
		sessionName = s.Name
	}

	w, err := memory.GetWorking(sessionName)
	if err != nil {
		return fmt.Errorf("getting working state: %w", err)
	}
	if w == nil {
		fmt.Printf("No memory state for session '%s'\n", sessionName)
		return nil
	}

	fmt.Printf("Session:  %s\n", w.SessionID)
	fmt.Printf("Repo:     %s\n", w.Repo)
	if w.WorktreePath != "" {
		fmt.Printf("Worktree: %s\n", w.WorktreePath)
	}
	if w.GitBranch != "" {
		fmt.Printf("Branch:   %s\n", w.GitBranch)
	}
	if w.CurrentTask != "" {
		fmt.Printf("Task:     %s\n", w.CurrentTask)
	}
	if w.LastSummary != "" {
		fmt.Printf("\nLast Summary:\n%s\n", w.LastSummary)
	}

	pending, _ := memory.PendingSummaryCount()
	if pending > 0 {
		fmt.Printf("\nPending summaries: %d\n", pending)
	}

	fmt.Printf("Last updated: %s\n", w.LastUpdated.Format("2006-01-02 15:04:05"))

	return nil
}

func sessionHistory(args []string) error {
	sessionName := ""
	n := 20

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			if i+1 < len(args) {
				n, _ = strconv.Atoi(args[i+1])
				i++
			}
		default:
			sessionName = args[i]
		}
	}

	if sessionName == "" {
		s, err := session.FindActiveSession()
		if err != nil {
			return fmt.Errorf("no active session (specify one): %w", err)
		}
		sessionName = s.Name
	}

	entries, err := memory.RecentEpisodic(sessionName, n)
	if err != nil {
		return fmt.Errorf("reading episodic log: %w", err)
	}

	if len(entries) == 0 {
		fmt.Printf("No episodic entries for '%s'\n", sessionName)
		return nil
	}

	fmt.Printf("Episodic log for '%s' (last %d):\n\n", sessionName, n)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		ts := e.Timestamp.Format("15:04:05")
		content := e.Content
		if len(content) > 120 {
			content = content[:117] + "..."
		}
		fmt.Printf("  [%s] %-15s %s\n", ts, e.Type, content)
	}
	return nil
}

func sessionClear(sessionName string) error {
	if sessionName == "" {
		s, err := session.FindActiveSession()
		if err != nil {
			return fmt.Errorf("no active session (specify one): %w", err)
		}
		sessionName = s.Name
	}

	memory.DeleteSessionEpisodic(sessionName)
	memory.DeleteWorking(sessionName)
	fmt.Printf("Cleared memory for '%s'\n", sessionName)
	return nil
}

func printSessionHelp() {
	fmt.Println(`bay session — Session lifecycle and state

Manage bay sessions: list, inspect, destroy, and view session memory.

Usage:
  bay session ls                     List all sessions with repo and worktree info.
  bay session kill <name>            Kill a session: destroys the tmux window, removes
                                     the worktree, cleans up memory, and deletes the
                                     session file. Cannot be undone.
  bay session note "text"            Append a note to session history. Use for
                                     breadcrumbs: decisions, dead ends, context.
  bay session show [session]         Show session state (tasks, summary, repo, branch).
  bay session history [session] [-n] Show the episodic log (newest last).
  bay session clear [session]        Wipe all memory for a session. Cannot be undone.`)
}
