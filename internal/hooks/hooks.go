package hooks

import (
	"time"

	"bay/internal/config"
	bayctx "bay/internal/context"
	"bay/internal/memory"
	"bay/internal/session"
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

	// Sync context files to worktree
	bayctx.SyncRulesToWorktree(workingDir, repoName)

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

// OnSessionDeactivate captures all dev pane buffers, queues summaries, syncs pane layout, and logs event.
func OnSessionDeactivate(sessionName, repoPath string, windowIdx int) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	if cfg.Memory.EpisodicLogging {
		memory.AppendEpisodic(sessionName, "deactivate", "session deactivated", "")
	}

	// Sync pane layout to session YAML
	syncPaneLayout(sessionName, windowIdx)

	if !cfg.Memory.AutoSummarize {
		return nil
	}

	// Capture all dev pane buffers
	buffers, err := baytmux.CaptureAllDevPanes(windowIdx, 100)
	if err != nil {
		return nil // Non-fatal: panes may already be gone
	}

	// Queue each buffer for async summarization
	for paneID, buffer := range buffers {
		if len(buffer) > 0 {
			memory.SummarizeAsync(sessionName, buffer, paneID)
		}
	}

	return nil
}

// syncPaneLayout snapshots the current tmux pane layout and persists it to session YAML.
func syncPaneLayout(sessionName string, windowIdx int) {
	s, err := session.Load(sessionName)
	if err != nil {
		return
	}

	panes, err := baytmux.SnapshotPaneLayout(windowIdx)
	if err != nil {
		return
	}

	var sessionPanes []session.Pane
	for _, p := range panes {
		paneType := "shell"
		if p.IsAgent {
			paneType = "agent"
		}

		sp := session.Pane{
			Type:    paneType,
			Cwd:     p.Cwd,
			Command: p.Command,
			PaneID:  p.PaneID,
		}

		sessionPanes = append(sessionPanes, sp)
	}

	s.Panes = sessionPanes
	session.Save(s)
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
