package config

// --- Worktree location enum values ---

const (
	// WorktreeManaged means worktrees live under ~/.bay/worktrees/{repo}/{branch}/.
	WorktreeManaged = "managed"

	// WorktreeAdjacent means worktrees are created next to the original repo.
	WorktreeAdjacent = "adjacent"
)

// DefaultContextBudget is the default token budget for context injection (in chars).
const DefaultContextBudget = 12000

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
	WorktreeLocation string `yaml:"worktree_location"`
}

// MemoryConfig controls context injection into agent prompts.
type MemoryConfig struct {
	Enabled          bool `yaml:"enabled"`
	ContextInjection bool `yaml:"context_injection"`
	ContextBudget    int  `yaml:"context_budget"`
}
