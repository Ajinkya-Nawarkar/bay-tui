package session

import "time"

// Pane represents a single pane within a session.
type Pane struct {
	Type    string `yaml:"type"`              // "shell" or "agent"
	Cwd     string `yaml:"cwd"`               // working directory
	Command string `yaml:"command,omitempty"`  // command running (e.g., "claude")
	PaneID          string `yaml:"pane_id,omitempty"`           // tmux pane ID (transient, for mapping)
	Title           string `yaml:"title,omitempty"`              // user-set pane label
	AgentSessionID string `yaml:"agent_session_id,omitempty"` // agent session UUID for resume
	TaskID         int    `yaml:"task_id,omitempty"`           // assigned task from tasks table
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
	Note           string    `yaml:"note,omitempty"`
}
