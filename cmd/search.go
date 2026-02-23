package cmd

import (
	"fmt"
	"os"

	"bay/internal/memory"
)

// Search runs FTS5 search across episodic history.
func Search(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: bay search \"query\" [--session name]")
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
