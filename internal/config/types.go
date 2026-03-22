package config

// --- Worktree location enum values ---

const (
	// WorktreeManaged means worktrees live under ~/.bay/worktrees/{repo}/{branch}/.
	WorktreeManaged = "managed"

	// WorktreeAdjacent means worktrees are created next to the original repo.
	WorktreeAdjacent = "adjacent"
)

// DefaultContextBudget is the default token budget for context injection (in chars).
// Tuned to fit comfortably within a single Claude prompt alongside other context.
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
	Shell            string `yaml:"shell"`             // e.g. "zsh", "bash", "fish"
	Agent            string `yaml:"agent"`             // e.g. "claude", "claude --dangerously-bypass-permissions"
	WorktreeLocation string `yaml:"worktree_location"` // WorktreeManaged or WorktreeAdjacent
}

// MemoryConfig controls which memory capabilities are enabled.
type MemoryConfig struct {
	Enabled          bool `yaml:"enabled"`            // master switch for all memory features
	EpisodicLogging  bool `yaml:"episodic_logging"`   // record pane snapshots to episodic table
	AutoSummarize    bool `yaml:"auto_summarize"`     // send pane snapshots through LLM summarization
	ContextInjection bool `yaml:"context_injection"`  // inject session context into agent prompts
	ContextBudget    int  `yaml:"context_budget"`     // max chars of context to inject (default: DefaultContextBudget)
}
