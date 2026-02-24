package config

// Config represents the global bay configuration stored in ~/.bay/config.yaml
type Config struct {
	Version  int          `yaml:"version"`
	ScanDirs []string     `yaml:"scan_dirs"`
	Defaults Defaults     `yaml:"defaults"`
	Memory   MemoryConfig `yaml:"memory"`
}

// Defaults holds default preferences for new sessions.
type Defaults struct {
	Shell            string `yaml:"shell"`
	Agent            string `yaml:"agent"`
	WorktreeLocation string `yaml:"worktree_location"` // "managed" or "adjacent"
}

// MemoryConfig controls which memory capabilities are enabled.
type MemoryConfig struct {
	Enabled          bool `yaml:"enabled"`
	EpisodicLogging  bool `yaml:"episodic_logging"`
	AutoSummarize    bool `yaml:"auto_summarize"`
	ContextInjection bool `yaml:"context_injection"`
	SiblingContext   bool `yaml:"sibling_context"`
	RulesInjection   bool `yaml:"rules_injection"`
	CrossRepoContext bool `yaml:"cross_repo_context"`
}
