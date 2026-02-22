package scanner

import (
	"os"
	"path/filepath"
	"sort"
)

// Repo represents a discovered git repository.
type Repo struct {
	Name string
	Path string
}

// Scan looks for git repos in the given directories (top-level only).
func Scan(dirs []string) []Repo {
	seen := make(map[string]bool)
	var repos []Repo

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			full := filepath.Join(dir, e.Name())
			gitDir := filepath.Join(full, ".git")
			if _, err := os.Stat(gitDir); err != nil {
				continue
			}
			if seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			repos = append(repos, Repo{
				Name: e.Name(),
				Path: full,
			})
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})
	return repos
}
