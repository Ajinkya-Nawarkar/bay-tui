package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bay/internal/config"
)

// SyncRulesToWorktree copies active rule files to <workingDir>/.claude/rules/
// and generates a <workingDir>/.claude/CLAUDE.md routing table.
// Only runs for worktree sessions (workingDir under ~/.bay/worktrees/).
func SyncRulesToWorktree(workingDir, repoName string) error {
	// Only sync for worktree sessions
	if !strings.HasPrefix(workingDir, config.WorktreesDir()) {
		return nil
	}

	activeRules, err := ActiveRules(repoName)
	if err != nil {
		return fmt.Errorf("getting active rules: %w", err)
	}

	claudeDir := filepath.Join(workingDir, ".claude")
	rulesDir := filepath.Join(claudeDir, "rules")

	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("creating .claude/rules/: %w", err)
	}

	// Copy each rule's content
	for _, r := range activeRules {
		content, err := ReadContent(r)
		if err != nil {
			continue // skip unreadable rules
		}
		dest := filepath.Join(rulesDir, r.Name+".md")
		if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing rule %s: %w", r.Name, err)
		}
	}

	// Generate CLAUDE.md routing table
	claudeMD := generateClaudeMD(activeRules)
	claudeMDPath := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(claudeMD), 0644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	return nil
}

// generateClaudeMD builds the CLAUDE.md routing table content.
func generateClaudeMD(activeRules []Rule) string {
	var b strings.Builder

	b.WriteString("# Session Context\n\n")
	b.WriteString("This session is managed by bay. **Before creating rules, logging notes, or managing\n")
	b.WriteString("memory, you MUST read `.claude/rules/bay-cli.md` for the correct commands and workflow.**\n")

	if len(activeRules) > 0 {
		b.WriteString("\n## Rules\n")
		for _, r := range activeRules {
			if r.Name == bayCLIRuleName {
				b.WriteString(fmt.Sprintf("- For bay CLI and memory system: see `.claude/rules/%s.md`\n", r.Name))
			} else {
				b.WriteString(fmt.Sprintf("- For %s: see `.claude/rules/%s.md`\n", r.Name, r.Name))
			}
		}
	}

	return b.String()
}
