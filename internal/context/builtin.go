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
and provides persistent memory.

## Available Commands

- ` + "`bay mem task \"description\"`" + ` — set the current task (persists across agent restarts)
- ` + "`bay mem note \"text\"`" + ` — log a note to session history
- ` + "`bay mem show`" + ` — view current session state (task, summary, repo, branch)
- ` + "`bay search \"query\"`" + ` — full-text search across session history
- ` + "`bay context add <name> <path>`" + ` — register a context file
- ` + "`bay context rm <name>`" + ` — remove a context file
- ` + "`bay context ls`" + ` — list all context files

## How Memory Works

Bay automatically captures your work when sessions switch. Summaries are generated
and injected when new agents start. Use ` + "`bay mem task`" + ` to set what you're working on
so future agents know the goal.
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
