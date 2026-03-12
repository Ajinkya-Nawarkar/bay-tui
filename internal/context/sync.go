package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SyncRulesToWorktree copies active context files to <workingDir>/.claude/rules/bay/
// and generates a <workingDir>/.claude/rules/bay/_index.md routing table.
// Bay owns the .claude/rules/bay/ subdirectory — user content elsewhere is untouched.
func SyncRulesToWorktree(workingDir, repoName string) error {
	activeFiles, err := ActiveRules(repoName)
	if err != nil {
		return fmt.Errorf("getting active context files: %w", err)
	}

	bayDir := filepath.Join(workingDir, ".claude", "rules", "bay")

	if err := os.MkdirAll(bayDir, 0755); err != nil {
		return fmt.Errorf("creating .claude/rules/bay/: %w", err)
	}

	// Clean bay dir entirely — removes stale files from deleted/disabled entries
	entries, _ := os.ReadDir(bayDir)
	for _, e := range entries {
		os.Remove(filepath.Join(bayDir, e.Name()))
	}

	// Copy each active file's content
	for _, f := range activeFiles {
		content, err := ReadContent(f)
		if err != nil {
			continue // skip unreadable files
		}
		dest := filepath.Join(bayDir, f.Name+".md")
		if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing context file %s: %w", f.Name, err)
		}
	}

	// Generate _index.md routing table
	indexContent := generateIndex(activeFiles)
	indexPath := filepath.Join(bayDir, "_index.md")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("writing _index.md: %w", err)
	}

	return nil
}

// generateIndex builds the _index.md routing table content, grouped by category.
func generateIndex(activeFiles []ContextFile) string {
	var b strings.Builder

	b.WriteString("# Bay-Managed Context\n\n")
	b.WriteString("This session is managed by bay. Use `bay ctx` commands to manage context files.\n")

	if len(activeFiles) == 0 {
		return b.String()
	}

	// Group by category
	groups := make(map[string][]ContextFile)
	var order []string
	for _, f := range activeFiles {
		cat := f.Category
		if cat == "" {
			cat = "rules"
		}
		if _, exists := groups[cat]; !exists {
			order = append(order, cat)
		}
		groups[cat] = append(groups[cat], f)
	}

	for _, cat := range order {
		// Capitalize first letter for display
		display := strings.ToUpper(cat[:1]) + cat[1:]
		b.WriteString(fmt.Sprintf("\n## %s\n", display))
		for _, f := range groups[cat] {
			if f.Name == bayCLIRuleName {
				b.WriteString(fmt.Sprintf("- bay CLI and memory system: see `%s.md`\n", f.Name))
			} else {
				b.WriteString(fmt.Sprintf("- %s: see `%s.md`\n", f.Name, f.Name))
			}
		}
	}

	return b.String()
}
