package session

import "time"

// Pane represents a single pane within a session.
type Pane struct {
	Type string `yaml:"type"` // "shell" or "claude"
	Cwd  string `yaml:"cwd"`
}

// Session represents a bay dev session.
type Session struct {
	Name           string    `yaml:"name"`
	Repo           string    `yaml:"repo"`
	RepoPath       string    `yaml:"repo_path"`
	WorkingDir     string    `yaml:"working_dir"`
	IsWorktree     bool      `yaml:"is_worktree"`
	WorktreeBranch string    `yaml:"worktree_branch,omitempty"`
	CreatedAt      time.Time `yaml:"created_at"`
	TmuxWindow     int       `yaml:"tmux_window,omitempty"`
	Panes          []Pane    `yaml:"panes"`
}
