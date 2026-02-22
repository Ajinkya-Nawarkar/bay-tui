package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anawarkar/bay/internal/scanner"
)

func TestScanFindsGitRepos(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake repos
	for _, name := range []string{"repo-a", "repo-b", "not-a-repo"} {
		dir := filepath.Join(tmpDir, name)
		os.MkdirAll(dir, 0755)
		if name != "not-a-repo" {
			os.MkdirAll(filepath.Join(dir, ".git"), 0755)
		}
	}

	repos := scanner.Scan([]string{tmpDir})

	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}

	names := make(map[string]bool)
	for _, r := range repos {
		names[r.Name] = true
	}

	if !names["repo-a"] {
		t.Error("repo-a not found")
	}
	if !names["repo-b"] {
		t.Error("repo-b not found")
	}
	if names["not-a-repo"] {
		t.Error("not-a-repo should not be found")
	}
}

func TestScanEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	repos := scanner.Scan([]string{tmpDir})
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestScanNonExistentDir(t *testing.T) {
	repos := scanner.Scan([]string{"/nonexistent/path/xyz"})
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestScanDeduplicates(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "my-repo")
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	// Scan same directory twice
	repos := scanner.Scan([]string{tmpDir, tmpDir})
	if len(repos) != 1 {
		t.Errorf("expected 1 repo (deduplicated), got %d", len(repos))
	}
}

func TestScanSorted(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"zebra", "alpha", "middle"} {
		os.MkdirAll(filepath.Join(tmpDir, name, ".git"), 0755)
	}

	repos := scanner.Scan([]string{tmpDir})
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0].Name != "alpha" || repos[1].Name != "middle" || repos[2].Name != "zebra" {
		t.Errorf("repos not sorted: %v", repos)
	}
}
