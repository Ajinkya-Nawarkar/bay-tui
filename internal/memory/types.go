package memory

import "time"

// EpisodicEntry represents a single event in the episodic log.
type EpisodicEntry struct {
	ID        int64
	SessionID string
	Type      string // "cmd", "llm", "pane_snapshot", "activate", "deactivate", "git_commit", "note", "summary"
	Content   string
	PaneID    string
	Timestamp time.Time
}

// WorkingState represents the live session state in working memory.
type WorkingState struct {
	SessionID    string
	Repo         string
	WorktreePath string
	GitBranch    string
	CurrentTask  string
	LastSummary  string
	ActiveSince  *time.Time
	LastUpdated  time.Time
}

// SiblingContext holds a summary from a sibling session in the same repo.
type SiblingContext struct {
	Session     string
	Branch      string
	LastSummary string
	LastUpdated time.Time
}
