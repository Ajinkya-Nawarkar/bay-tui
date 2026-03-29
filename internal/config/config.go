// Package config manages ~/.bay/config.yaml — the global bay configuration file.
//
// Load reads the config from disk and unmarshals it; Save marshals and writes
// atomically so a crash mid-write never corrupts the file. BayDir() returns the
// root directory (~/.bay/) and every other path helper (ConfigPath, SessionsDir,
// WorktreesDir, DBPath, etc.) is derived from it.
//
// EnsureDirs creates the full directory skeleton (~/.bay/sessions/, worktrees/,
// logs/, pane-agents/) on first run, making it safe for other packages to assume
// these directories exist. DefaultConfig() returns sensible defaults for a fresh
// install so the setup wizard has something to write.
package config

import (
	"fmt"
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
		filepath.Join(BayDir(), "logs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
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

// Validate checks a Config for obviously invalid values.
func Validate(cfg *Config) error {
	if cfg.Defaults.WorktreeLocation != "" &&
		cfg.Defaults.WorktreeLocation != WorktreeManaged &&
		cfg.Defaults.WorktreeLocation != WorktreeAdjacent {
		return fmt.Errorf("invalid worktree_location %q (want %q or %q)",
			cfg.Defaults.WorktreeLocation, WorktreeManaged, WorktreeAdjacent)
	}
	if cfg.Memory.ContextBudget < 0 {
		return fmt.Errorf("context_budget must be non-negative, got %d", cfg.Memory.ContextBudget)
	}
	return nil
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
	return os.WriteFile(ConfigPath(), data, 0o644)
}

// DBPath returns the path to ~/.bay/bay.db
func DBPath() string {
	return filepath.Join(BayDir(), "bay.db")
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version:  1,
		ScanDirs: []string{},
		Defaults: Defaults{
			Shell:            "zsh",
			Agent:            "claude",
			WorktreeLocation: WorktreeManaged,
		},
		Memory: DefaultMemoryConfig(),
	}
}

// DefaultMemoryConfig returns memory config with all features enabled.
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		Enabled:          true,
		ContextInjection: true,
		ContextBudget:    DefaultContextBudget,
	}
}
