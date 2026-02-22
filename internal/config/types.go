package config

// Config represents the global bay configuration stored in ~/.bay/config.yaml
type Config struct {
	Version  int      `yaml:"version"`
	ScanDirs []string `yaml:"scan_dirs"`
	Defaults Defaults `yaml:"defaults"`
}

// Defaults holds default preferences for new sessions.
type Defaults struct {
	Shell             string `yaml:"shell"`
	Agent             string `yaml:"agent"`
	WorktreeLocation  string `yaml:"worktree_location"` // "managed" or "adjacent"
}
