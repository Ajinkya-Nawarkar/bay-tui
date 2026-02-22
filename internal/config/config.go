package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BayDir returns the path to ~/.bay/
func BayDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bay")
}

// ConfigPath returns the path to ~/.bay/config.yaml
func ConfigPath() string {
	return filepath.Join(BayDir(), "config.yaml")
}

// SessionsDir returns the path to ~/.bay/sessions/
func SessionsDir() string {
	return filepath.Join(BayDir(), "sessions")
}

// WorktreesDir returns the path to ~/.bay/worktrees/
func WorktreesDir() string {
	return filepath.Join(BayDir(), "worktrees")
}

// EnsureDirs creates the ~/.bay/ directory structure.
func EnsureDirs() error {
	dirs := []string{
		BayDir(),
		SessionsDir(),
		WorktreesDir(),
		filepath.Join(BayDir(), "agents"),
		filepath.Join(BayDir(), "logs"),
		filepath.Join(BayDir(), "plugins"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

// Exists returns true if the config file exists.
func Exists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}

// Load reads and parses ~/.bay/config.yaml.
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config to ~/.bay/config.yaml.
func Save(cfg *Config) error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version:  1,
		ScanDirs: []string{},
		Defaults: Defaults{
			Shell:            "zsh",
			Agent:            "claude",
			WorktreeLocation: "managed",
		},
	}
}
