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
and provides persistent memory. Resources (rules, skills, agents, plugins) live in
` + "`~/.bay/`" + ` and are discovered lazily via ` + "`~/.bay/CLAUDE.md`" + `.

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

## Resource Discovery

Resources are organized in ` + "`~/.bay/{type}/`" + ` directories (rules, skills, agents, plugins).
Each directory has an ` + "`index.yaml`" + ` catalog. Read ` + "`~/.bay/CLAUDE.md`" + ` for the full directory listing.

## Project Context

Store project-specific knowledge in ` + "`~/.bay/context/projects/<project-name>/`" + `.
This is the right place for architecture docs, design decisions, status, and conventions
that should persist across sessions and be available to all agents working on the project.
`

// ContextFilesDir returns the path to ~/.bay/rules/
func ContextFilesDir() string {
	return filepath.Join(config.BayDir(), "rules")
}

// EnsureBuiltinRules writes the bay-cli rule file and ensures it's registered in the DB.
// The rule file is always overwritten to pick up content changes from new bay versions.
func EnsureBuiltinRules(d *sql.DB) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	// Ensure rules dir exists
	rulesDir := ContextFilesDir()
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("creating rules dir: %w", err)
	}

	// Always overwrite the rule file — bay-cli is bay-managed, not user-edited
	rulePath := filepath.Join(rulesDir, bayCLIRuleName+".md")
	if err := os.WriteFile(rulePath, []byte(bayCLIRuleContent), 0644); err != nil {
		return fmt.Errorf("writing bay-cli rule: %w", err)
	}

	// Register in DB if not already present
	var exists bool
	if err := d.QueryRow(`SELECT 1 FROM context_files WHERE name = ? LIMIT 1`, bayCLIRuleName).Scan(&exists); err == nil && exists {
		return nil
	}

	if err := AddDB(d, bayCLIRuleName, rulePath, "global", "rules", "rules", "Bay CLI and memory system reference"); err != nil {
		return fmt.Errorf("registering bay-cli rule: %w", err)
	}

	return nil
}
