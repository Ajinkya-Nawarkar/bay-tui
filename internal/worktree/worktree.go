package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anawarkar/bay/internal/config"
)

// WorktreePath returns the managed worktree path for a repo and branch.
func WorktreePath(repoName, branch string) string {
	return filepath.Join(config.WorktreesDir(), repoName, branch)
}

// Create creates a new git worktree for the given repo and branch.
// If the branch doesn't exist, it creates it from HEAD.
func Create(repoPath, repoName, branch string) (string, error) {
	wtPath := WorktreePath(repoName, branch)

	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", fmt.Errorf("creating worktree directory: %w", err)
	}

	// Try adding worktree with existing branch first
	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Branch might not exist, try creating it
		cmd = exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branch, wtPath)
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return "", fmt.Errorf("git worktree add: %s\n%s", string(out), string(out2))
		}
	}

	return wtPath, nil
}

// Remove removes a git worktree.
func Remove(repoPath, repoName, branch string) error {
	wtPath := WorktreePath(repoName, branch)

	cmd := exec.Command("git", "-C", repoPath, "worktree", "remove", wtPath, "--force")
	if out, err := cmd.CombinedOutput(); err != nil {
		// If git worktree remove fails, try manual cleanup
		_ = os.RemoveAll(wtPath)
		// Prune stale worktree entries
		exec.Command("git", "-C", repoPath, "worktree", "prune").Run()
		_ = out
	}

	// Clean up empty parent directory
	parent := filepath.Dir(wtPath)
	entries, _ := os.ReadDir(parent)
	if len(entries) == 0 {
		os.Remove(parent)
	}

	return nil
}

// ListEntry represents a single worktree.
type ListEntry struct {
	Path   string
	Branch string
	Bare   bool
}

// List returns all worktrees for a given repo.
func List(repoPath string) ([]ListEntry, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var entries []ListEntry
	var current ListEntry

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				entries = append(entries, current)
			}
			current = ListEntry{}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			// refs/heads/branch-name -> branch-name
			current.Branch = filepath.Base(ref)
		} else if line == "bare" {
			current.Bare = true
		}
	}
	if current.Path != "" {
		entries = append(entries, current)
	}

	return entries, nil
}
