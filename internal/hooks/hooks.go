package hooks

import (
	"time"

	"bay/internal/config"
	"bay/internal/memory"
	baytmux "bay/internal/tmux"
)

// loadConfig loads config with defaults fallback.
func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

// OnSessionCreate inserts working_state row and logs "activate" to episodic.
func OnSessionCreate(sessionName, repoName, workingDir string) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	now := time.Now()
	w := &memory.WorkingState{
		SessionID:    sessionName,
		Repo:         repoName,
		WorktreePath: workingDir,
		ActiveSince:  &now,
	}

	if err := memory.UpsertWorking(w); err != nil {
		return err
	}

	if cfg.Memory.EpisodicLogging {
		memory.AppendEpisodic(sessionName, "activate", "session created: "+repoName, "")
	}

	return nil
}

// OnSessionActivate updates working_state with current branch/worktree and logs event.
func OnSessionActivate(sessionName, repoName, workingDir string) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	now := time.Now()
	w := &memory.WorkingState{
		SessionID:    sessionName,
		Repo:         repoName,
		WorktreePath: workingDir,
		ActiveSince:  &now,
	}

	// Try to preserve existing fields (task, summary) from prior state
	existing, err := memory.GetWorking(sessionName)
	if err == nil && existing != nil {
		w.CurrentTask = existing.CurrentTask
		w.LastSummary = existing.LastSummary
		w.ClaudeSessionID = existing.ClaudeSessionID
		w.GitBranch = existing.GitBranch
	}

	if err := memory.UpsertWorking(w); err != nil {
		return err
	}

	if cfg.Memory.EpisodicLogging {
		memory.AppendEpisodic(sessionName, "activate", "session activated", "")
	}

	return nil
}

// OnSessionDeactivate captures all dev pane buffers, queues summaries, and logs event.
func OnSessionDeactivate(sessionName, repoPath string, windowIdx int) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	if cfg.Memory.EpisodicLogging {
		memory.AppendEpisodic(sessionName, "deactivate", "session deactivated", "")
	}

	if !cfg.Memory.AutoSummarize {
		return nil
	}

	// Capture all dev pane buffers
	buffers, err := baytmux.CaptureAllDevPanes(windowIdx, 100)
	if err != nil {
		return nil // Non-fatal: panes may already be gone
	}

	// Queue each buffer for async summarization
	for _, buffer := range buffers {
		if len(buffer) > 0 {
			memory.SummarizeAsync(sessionName, buffer)
		}
	}

	return nil
}

// OnSessionDelete removes all DB rows for the session.
func OnSessionDelete(sessionName string) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	memory.DeleteSessionEpisodic(sessionName)
	memory.DeleteWorking(sessionName)
	return nil
}

// OnSessionRename updates session_id across all tables.
func OnSessionRename(oldName, newName string) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	memory.RenameWorking(oldName, newName)

	// Rename episodic entries
	d, err := memory.GetDB()
	if err != nil {
		return nil
	}
	d.Exec(`UPDATE episodic SET session_id = ? WHERE session_id = ?`, newName, oldName)
	d.Exec(`UPDATE pending_summaries SET session_id = ? WHERE session_id = ?`, newName, oldName)

	return nil
}
