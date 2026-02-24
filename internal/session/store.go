package session

import (
	"os"
	"path/filepath"
	"strings"

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
	return os.WriteFile(sessionPath(s.Name), data, 0644)
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

// List returns all saved sessions.
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
		sessions = append(sessions, s)
	}
	return sessions, nil
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
