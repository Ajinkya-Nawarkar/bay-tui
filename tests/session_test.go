package tests

import (
	"os"
	"testing"
	"time"

	"github.com/anawarkar/bay/internal/config"
	"github.com/anawarkar/bay/internal/session"
)

func setupSessionTest(t *testing.T) func() {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	config.EnsureDirs()
	return func() { os.Setenv("HOME", origHome) }
}

func TestSessionSaveAndLoad(t *testing.T) {
	cleanup := setupSessionTest(t)
	defer cleanup()

	s := &session.Session{
		Name:       "test-session",
		Repo:       "TestRepo",
		RepoPath:   "/tmp/TestRepo",
		WorkingDir: "/tmp/TestRepo",
		CreatedAt:  time.Now(),
		Panes: []session.Pane{
			{Type: "shell", Cwd: "/tmp/TestRepo"},
		},
	}

	if err := session.Save(s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := session.Load("test-session")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != "test-session" {
		t.Errorf("expected name test-session, got %s", loaded.Name)
	}
	if loaded.Repo != "TestRepo" {
		t.Errorf("expected repo TestRepo, got %s", loaded.Repo)
	}
	if len(loaded.Panes) != 1 {
		t.Errorf("expected 1 pane, got %d", len(loaded.Panes))
	}
}

func TestSessionList(t *testing.T) {
	cleanup := setupSessionTest(t)
	defer cleanup()

	// Create two sessions
	for _, name := range []string{"alpha", "beta"} {
		s := &session.Session{
			Name:       name,
			Repo:       "Repo",
			RepoPath:   "/tmp/Repo",
			WorkingDir: "/tmp/Repo",
			CreatedAt:  time.Now(),
		}
		session.Save(s)
	}

	list, err := session.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(list))
	}
}

func TestSessionDelete(t *testing.T) {
	cleanup := setupSessionTest(t)
	defer cleanup()

	s := &session.Session{
		Name:       "to-delete",
		Repo:       "Repo",
		RepoPath:   "/tmp/Repo",
		WorkingDir: "/tmp/Repo",
		CreatedAt:  time.Now(),
	}
	session.Save(s)

	if err := session.Delete("to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := session.Load("to-delete")
	if err == nil {
		t.Error("expected error loading deleted session")
	}
}

func TestSessionRename(t *testing.T) {
	cleanup := setupSessionTest(t)
	defer cleanup()

	s := &session.Session{
		Name:       "old-name",
		Repo:       "Repo",
		RepoPath:   "/tmp/Repo",
		WorkingDir: "/tmp/Repo",
		CreatedAt:  time.Now(),
	}
	session.Save(s)

	if err := session.Rename("old-name", "new-name"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	loaded, err := session.Load("new-name")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Name != "new-name" {
		t.Errorf("expected new-name, got %s", loaded.Name)
	}

	_, err = session.Load("old-name")
	if err == nil {
		t.Error("old session should not exist")
	}
}
