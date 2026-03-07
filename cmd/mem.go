package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"bay/internal/config"
	"bay/internal/memory"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
)

// Mem handles the `bay mem` subcommands.
func Mem(args []string) error {
	if len(args) == 0 {
		printMemHelp()
		return nil
	}

	switch args[0] {
	case "show":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return memShow(sessionName)

	case "task":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay mem task \"description\"")
			return nil
		}
		return memTask(strings.Join(args[1:], " "))

	case "note":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay mem note \"text\"")
			return nil
		}
		return memNote(strings.Join(args[1:], " "))

	case "log":
		return memLog(args[1:])

	case "capture":
		if len(args) < 2 {
			return fmt.Errorf("usage: bay mem capture <pane-id>")
		}
		return memCapture(args[1])

	case "record":
		if len(args) < 4 {
			return fmt.Errorf("usage: bay mem record <type> <pane-id> <data>")
		}
		return memRecord(args[1], args[2], strings.Join(args[3:], " "))

	case "clear":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return memClear(sessionName)

	case "config":
		return memConfig(args[1:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown mem command: %s\n", args[0])
		printMemHelp()
		return nil
	}
}

func memShow(sessionName string) error {
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

func memTask(task string) error {
	s, err := session.FindActiveSession()
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}

	// Ensure working state exists
	w, err := memory.GetWorking(s.Name)
	if err != nil {
		return err
	}
	if w == nil {
		w = &memory.WorkingState{SessionID: s.Name, Repo: s.Repo, WorktreePath: s.WorkingDir}
		if err := memory.UpsertWorking(w); err != nil {
			return fmt.Errorf("creating working state: %w", err)
		}
	}

	if err := memory.SetTask(s.Name, task); err != nil {
		return fmt.Errorf("setting task: %w", err)
	}

	fmt.Printf("Task set: %s\n", task)
	return nil
}

func memNote(text string) error {
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

func memLog(args []string) error {
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
	// Reverse to show oldest first
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

func memCapture(paneID string) error {
	// Find which session owns this pane by checking all sessions' windows
	sessions, err := session.List()
	if err != nil {
		return err
	}

	// Capture the buffer first
	buffer, err := baytmux.CapturePaneBuffer(paneID, 100)
	if err != nil {
		return fmt.Errorf("capturing pane %s: %w", paneID, err)
	}

	if len(strings.TrimSpace(buffer)) == 0 {
		return nil
	}

	// Try to find the session this pane belongs to
	sessionName := "unknown"
	for _, s := range sessions {
		if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
			sessionName = s.Name
			break
		}
	}

	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if cfg.Memory.AutoSummarize {
		return memory.SummarizeAsync(sessionName, buffer, paneID)
	}

	return memory.AppendEpisodic(sessionName, "pane_snapshot", buffer, paneID)
}

func memRecord(eventType, paneID, data string) error {
	s, err := session.FindActiveSession()
	if err != nil {
		return memory.AppendEpisodic("unknown", eventType, data, paneID)
	}
	return memory.AppendEpisodic(s.Name, eventType, data, paneID)
}

func memClear(sessionName string) error {
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

func memConfig(args []string) error {
	if len(args) == 0 {
		// Show current config
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
		fmt.Fprintln(os.Stderr, "Usage: bay mem config <feature> on|off|<value>")
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

func printMemHelp() {
	fmt.Println(`bay mem — Session memory management

Usage:
  bay mem show [session]         Show working state for session
  bay mem task "description"     Set current task in working memory
  bay mem note "text"            Add a note to episodic log
  bay mem log [session] [-n 50]  Show episodic log (time travel)
  bay mem clear [session]        Clear all memory for a session
  bay mem config [feature] [on|off]  Toggle memory features

Internal (used by tmux hooks):
  bay mem capture <pane-id>      Capture pane buffer + queue summarize
  bay mem record <type> <pane-id> <data>  Append to episodic log`)
}

func parseBool(s string) bool {
	return s == "on" || s == "true"
}
