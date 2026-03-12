package hooks

import (
	"sync"
	"time"

	"bay/internal/config"
	bayctx "bay/internal/context"
	"bay/internal/memory"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
)

// Debounce state: prevents rapid duplicate captures for the same session.
var (
	lastCaptureMu   sync.Mutex
	lastCaptureTime = make(map[string]time.Time)
)

// debounceDuration is how long to wait before allowing another capture for the same session.
const debounceDuration = 5 * time.Second

// ShouldCapture returns true if enough time has passed since the last capture for this session.
// Updates the timestamp on success.
func ShouldCapture(sessionName string) bool {
	lastCaptureMu.Lock()
	defer lastCaptureMu.Unlock()

	now := time.Now()
	if last, ok := lastCaptureTime[sessionName]; ok {
		if now.Sub(last) < debounceDuration {
			return false
		}
	}
	lastCaptureTime[sessionName] = now
	return true
}

// ShouldCaptureWithDuration is like ShouldCapture but accepts a custom duration (for testing).
func ShouldCaptureWithDuration(sessionName string, dur time.Duration) bool {
	lastCaptureMu.Lock()
	defer lastCaptureMu.Unlock()

	now := time.Now()
	if last, ok := lastCaptureTime[sessionName]; ok {
		if now.Sub(last) < dur {
			return false
		}
	}
	lastCaptureTime[sessionName] = now
	return true
}

// ResetDebounce clears debounce state (for testing).
func ResetDebounce() {
	lastCaptureMu.Lock()
	defer lastCaptureMu.Unlock()
	lastCaptureTime = make(map[string]time.Time)
}

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

// OnSessionDeactivate syncs pane layout and logs the deactivation event.
// Agent panes are resumed via claude session IDs on cold boot, so no buffer
// capture or LLM summarization is needed on session switch.
func OnSessionDeactivate(sessionName, repoPath string, windowIdx int) error {
	cfg := loadConfig()
	if !cfg.Memory.Enabled {
		return nil
	}

	if cfg.Memory.EpisodicLogging {
		memory.AppendEpisodic(sessionName, "deactivate", "session deactivated", "")
	}

	// Sync pane layout (including claude session IDs) to session YAML
	SyncPaneLayout(sessionName, windowIdx)

	return nil
}

// SyncPaneLayout snapshots the current tmux pane layout and persists it to session YAML.
// Preserves AgentSessionID from existing YAML data (tmux doesn't know about it).
func SyncPaneLayout(sessionName string, windowIdx int) {
	s, err := session.Load(sessionName)
	if err != nil {
		return
	}

	panes, err := baytmux.SnapshotPaneLayout(windowIdx)
	if err != nil {
		return
	}

	// Build a lookup of existing claude session IDs by tmux pane ID
	existingIDs := make(map[string]string)
	for _, p := range s.Panes {
		if p.PaneID != "" && p.AgentSessionID != "" {
			existingIDs[p.PaneID] = p.AgentSessionID
		}
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
			Title:   p.Title,
		}

		// Preserve claude session ID from existing YAML data
		if id, ok := existingIDs[p.PaneID]; ok {
			sp.AgentSessionID = id
		}

		sessionPanes = append(sessionPanes, sp)
	}

	s.Panes = sessionPanes
	session.Save(s)
}

// CleanOrphanWindows kills tmux windows that don't belong to any saved session.
// Window 0 is always preserved as the topbar's fallback.
func CleanOrphanWindows() {
	sessions, err := session.List()
	if err != nil {
		return
	}

	owned := make(map[int]bool)
	for _, s := range sessions {
		if s.TmuxWindow != 0 {
			owned[s.TmuxWindow] = true
		}
	}

	for _, idx := range baytmux.ListWindowIndices() {
		if idx == 0 || owned[idx] {
			continue
		}
		baytmux.KillWindow(idx)
	}
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
