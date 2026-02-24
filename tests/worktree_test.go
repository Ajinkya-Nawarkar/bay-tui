package tests

import (
	"os"
	"strings"
	"testing"

	"bay/internal/worktree"
)

func TestWorktreePath(t *testing.T) {
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", "/tmp/testhome")
	defer os.Setenv("HOME", origHome)

	path := worktree.WorktreePath("MyRepo", "feature-x")
	if !strings.Contains(path, ".bay/worktrees/MyRepo/feature-x") {
		t.Errorf("unexpected path: %s", path)
	}
}

// Note: Integration tests for git worktree operations require an actual git repo.
// This tests the path computation logic.
