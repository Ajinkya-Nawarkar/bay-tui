// Package session manages bay sessions as individual YAML files in ~/.bay/sessions/.
//
// Each session maps to one tmux window and tracks repo, panes, worktree info.
// FindActiveSession detects the current session from the tmux window index.
// Marker files (.active-session, .created-session) coordinate between TUI and CLI.
package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bay/internal/config"
	"gopkg.in/yaml.v3"
)

// sessionPath returns the path for a session YAML file.
func sessionPath(name string) string {
	return filepath.Join(config.SessionsDir(), name+".yaml")
}

// Save writes a session to ~/.bay/sessions/{name}.yaml.
func Save(s *Session) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(sessionPath(s.Name), data, 0o644)
}

// Load reads a session from its YAML file.
func Load(name string) (*Session, error) {
	data, err := os.ReadFile(sessionPath(name))
	if err != nil {
		return nil, err
	}
	var s Session
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete removes a session YAML file.
func Delete(name string) error {
	return os.Remove(sessionPath(name))
}

// List returns all saved sessions, excluding archived ones.
func List() ([]*Session, error) {
	dir := config.SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []*Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		s, err := Load(name)
		if err != nil {
			continue
		}
		if s.IsArchived() {
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// ListArchived returns only archived sessions, sorted by ArchivedAt descending.
func ListArchived() ([]*Session, error) {
	dir := config.SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []*Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		s, err := Load(name)
		if err != nil {
			continue
		}
		if !s.IsArchived() {
			continue
		}
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ArchivedAt.After(sessions[j].ArchivedAt)
	})
	return sessions, nil
}

// Archive marks a session as archived.
func Archive(name string) error {
	s, err := Load(name)
	if err != nil {
		return err
	}
	s.ArchivedAt = time.Now()
	s.TmuxWindow = 0
	return Save(s)
}

// Unarchive restores an archived session.
func Unarchive(name string) error {
	s, err := Load(name)
	if err != nil {
		return err
	}
	s.ArchivedAt = time.Time{}
	return Save(s)
}

// Rename changes a session's name and moves its YAML file.
func Rename(oldName, newName string) error {
	s, err := Load(oldName)
	if err != nil {
		return err
	}
	if err := Delete(oldName); err != nil {
		return err
	}
	s.Name = newName
	return Save(s)
}
