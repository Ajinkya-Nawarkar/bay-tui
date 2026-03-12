package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	bayctx "bay/internal/context"
	"bay/internal/config"
	"bay/internal/memory"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
)

// Ctx handles the `bay ctx` subcommands — everything about what agents know.
// Merges the old `bay mem`, `bay context`, and `bay search` commands.
func Ctx(args []string) error {
	if len(args) == 0 {
		return Context()
	}

	switch args[0] {
	// --- Memory / working state ---
	case "show":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return ctxShow(sessionName)

	case "task":
		fmt.Fprintln(os.Stderr, "bay ctx task has moved → use bay task instead")
		fmt.Fprintln(os.Stderr, "  bay task \"description\"     Create a task")
		fmt.Fprintln(os.Stderr, "  bay task ls                List tasks")
		fmt.Fprintln(os.Stderr, "  bay task done <id>         Mark done")
		return nil

	case "note":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: bay ctx note \"text\"")
			return nil
		}
		return ctxNote(strings.Join(args[1:], " "))

	case "history":
		return ctxHistory(args[1:])

	case "search":
		return ctxSearch(args[1:])

	case "clear":
		sessionName := ""
		if len(args) > 1 {
			sessionName = args[1]
		}
		return ctxClear(sessionName)

	case "config":
		return ctxConfig(args[1:])

	// --- Internal (tmux hooks) ---
	case "capture":
		if len(args) < 2 {
			return fmt.Errorf("usage: bay ctx capture <pane-id>")
		}
		return ctxCapture(args[1])

	case "record":
		if len(args) < 4 {
			return fmt.Errorf("usage: bay ctx record <type> <pane-id> <data>")
		}
		return ctxRecord(args[1], args[2], strings.Join(args[3:], " "))

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
// Memory / working state handlers
// ---------------------------------------------------------------------------

func ctxShow(sessionName string) error {
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

func ctxTask(task string) error {
	s, err := session.FindActiveSession()
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}

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

func ctxNote(text string) error {
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

func ctxHistory(args []string) error {
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

func ctxClear(sessionName string) error {
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
// Internal handlers (tmux hooks)
// ---------------------------------------------------------------------------

func ctxCapture(paneID string) error {
	sessions, err := session.List()
	if err != nil {
		return err
	}

	buffer, err := baytmux.CapturePaneBuffer(paneID, 100)
	if err != nil {
		return fmt.Errorf("capturing pane %s: %w", paneID, err)
	}

	if len(strings.TrimSpace(buffer)) == 0 {
		return nil
	}

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

func ctxRecord(eventType, paneID, data string) error {
	s, err := session.FindActiveSession()
	if err != nil {
		return memory.AppendEpisodic("unknown", eventType, data, paneID)
	}
	return memory.AppendEpisodic(s.Name, eventType, data, paneID)
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

	fmt.Printf("%-20s %-10s %-8s %-15s %s\n", "NAME", "CATEGORY", "STATUS", "SCOPE", "PATH")
	for _, f := range list {
		status := "on"
		if !f.Enabled {
			status = "off"
		}
		cat := f.Category
		if cat == "" {
			cat = "rules"
		}
		fmt.Printf("%-20s %-10s %-8s %-15s %s\n", f.Name, cat, status, f.Scope, f.Path)
	}
	return nil
}

func ctxAdd(args []string) error {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: bay ctx add <name> <path> [--scope S] [--category C]")
		return nil
	}

	name := args[0]
	path := args[1]
	scope := "global"
	category := "rules"

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
	}

	if err := bayctx.Add(name, path, scope, category); err != nil {
		return fmt.Errorf("adding context file: %w", err)
	}

	fmt.Printf("Added '%s' (%s, %s) → %s\n", name, category, scope, path)
	syncAllWorktreeSessions()
	return nil
}

func ctxRm(name string) error {
	if err := bayctx.Remove(name); err != nil {
		return fmt.Errorf("removing context file: %w", err)
	}
	fmt.Printf("Removed '%s'\n", name)
	syncAllWorktreeSessions()
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

	syncAllWorktreeSessions()
	return nil
}

func ctxSync() error {
	fmt.Println("Syncing context files to all worktree sessions...")
	syncAllWorktreeSessions()
	fmt.Println("Done.")
	return nil
}

// syncAllWorktreeSessions re-syncs context files to all active worktree sessions.
func syncAllWorktreeSessions() {
	sessions, err := session.List()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.IsWorktree {
			bayctx.SyncRulesToWorktree(s.WorkingDir, s.Repo)
		}
	}
}

func parseBool(s string) bool {
	return s == "on" || s == "true"
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func printCtxHelp() {
	fmt.Println(`bay ctx — Agent context and session memory

Tells agents what they need to know: current task, session history,
context files, and cross-session search. Also the internal plumbing
that captures pane output for automatic summarization.

Usage:

  Working State
    bay ctx show [session]             Show session state (tasks, summary, repo, branch).
                                       Use to check what an agent will see on startup.
    bay ctx note "text"                Append a note to the episodic log. Use for breadcrumbs
                                       that future agents or sessions should know about.

  Tasks (see bay task --help for full reference)
    bay task "description"             Create a task in the current session.
    bay task ls                        List all tasks with status.

  History & Search
    bay ctx history [session] [-n 50]  Show the episodic log (newest last). Useful for
                                       reviewing what happened in a session over time.
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
    bay ctx sync                       Re-sync context files to all worktree sessions.
                                       Run after adding/removing files if agents are active.

  Configuration
    bay ctx config                     Show current memory feature settings.
    bay ctx config <feature> on|off    Toggle: enabled, episodic_logging, auto_summarize,
                                       context_injection. Also: context_budget <int>.
    bay ctx clear [session]            Wipe all memory (episodic + working state) for a
                                       session. Cannot be undone.

  Internal (called by tmux hooks — not for direct use)
    bay ctx capture <pane-id>          Capture pane buffer on exit, queue for summarization.
    bay ctx record <type> <pane-id> <data>
                                       Append a typed event to the episodic log.`)
}
