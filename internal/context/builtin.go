package context

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"bay/internal/config"
	"bay/internal/db"
)

const bayCLIRuleName = "bay-cli"

const bayCLIRuleContent = `# Bay Session Manager

You are running inside a bay-managed session. Bay tracks your work across sessions
and provides persistent memory that is automatically injected when you start.

## Commands

### Tasks — structured task tracking per session

- ` + "`bay task \"description\"`" + ` — create a task. Tasks persist across agent restarts
  and appear in every new agent's context. Always create tasks when starting work.
- ` + "`bay task add \"desc\" [-p N]`" + ` — add a subtask, optionally under task #N.
- ` + "`bay task ls`" + ` — list all tasks with status.
- ` + "`bay task done <id>`" + ` / ` + "`bay task doing <id>`" + ` — update task status.
- ` + "`bay task assign <id>`" + ` — assign the current pane to a task. The context
  will show which task this agent is responsible for.

### Working State — session context and notes

- ` + "`bay ctx note \"text\"`" + ` — log a note to session history. Use for breadcrumbs:
  decisions made, dead ends hit, things the next agent should know.
- ` + "`bay ctx show`" + ` — view current session state (tasks, summary, repo, branch).
  Run this when you need to understand what was happening before you started.

### Search & History — find past work across all sessions

- ` + "`bay ctx search \"query\"`" + ` — full-text search across all session history.
  Finds terminal output, notes, and summaries from any session.
- ` + "`bay ctx history [-n 50]`" + ` — show the episodic log for this session.

### Context Files — documents injected into every agent in a session

- ` + "`bay ctx files`" + ` — list all registered context files and their status.
- ` + "`bay ctx add <name> <path>`" + ` — register a file for injection (design docs,
  API specs, coding standards). Use --scope repo:<name> to limit to one repo.
- ` + "`bay ctx rm <name>`" + ` — remove a registered context file.

## How Memory Works

Bay captures pane output when sessions switch. Summaries are generated and injected
into your context automatically on startup. Tasks created with ` + "`bay task`" + ` are the
most prominent piece of context — always keep them current.
`

// ContextFilesDir returns the path to ~/.bay/rules/
func ContextFilesDir() string {
	return filepath.Join(config.BayDir(), "rules")
}

// EnsureBuiltinRules checks if the bay-cli rule exists in the DB. If not, writes
// the rule file and registers it as a global context file.
func EnsureBuiltinRules(d *sql.DB) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	// Check if bay-cli rule already exists
	var exists bool
	if err := d.QueryRow(`SELECT 1 FROM context_files WHERE name = ? LIMIT 1`, bayCLIRuleName).Scan(&exists); err == nil && exists {
		return nil
	}

	// Ensure rules dir exists
	rulesDir := ContextFilesDir()
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("creating rules dir: %w", err)
	}

	// Write the rule file
	rulePath := filepath.Join(rulesDir, bayCLIRuleName+".md")
	if err := os.WriteFile(rulePath, []byte(bayCLIRuleContent), 0644); err != nil {
		return fmt.Errorf("writing bay-cli rule: %w", err)
	}

	// Register in DB
	if err := AddDB(d, bayCLIRuleName, rulePath, "global", "rules"); err != nil {
		return fmt.Errorf("registering bay-cli rule: %w", err)
	}

	return nil
}
