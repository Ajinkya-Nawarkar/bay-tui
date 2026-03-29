// Package hooks coordinates session lifecycle events.
//
// SyncPaneLayout snapshots tmux pane layout to session YAML, preserving agent session IDs.
// Debounce prevents rapid duplicate captures when windows switch quickly.
package hooks

import (
	"sync"
	"time"

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

// OnSessionCreate is a no-op placeholder for session creation events.
func OnSessionCreate(sessionName, repoName, workingDir string) error {
	return nil
}

// OnSessionActivate is a no-op placeholder for session activation events.
func OnSessionActivate(sessionName, repoName, workingDir string) error {
	return nil
}

// OnSessionDeactivate syncs pane layout on session switch.
func OnSessionDeactivate(sessionName, repoPath string, windowIdx int) error {
	SyncPaneLayout(sessionName, windowIdx)
	return nil
}

// SyncPaneLayout snapshots the current tmux pane layout and persists it to session YAML.
func SyncPaneLayout(sessionName string, windowIdx int) {
	s, err := session.Load(sessionName)
	if err != nil {
		return
	}

	panes, err := baytmux.SnapshotPaneLayout(windowIdx)
	if err != nil {
		return
	}

	existingIDs := make(map[string]string)
	existingIDsByIdx := make(map[int]string)
	for i, p := range s.Panes {
		if p.AgentSessionID != "" {
			if p.PaneID != "" {
				existingIDs[p.PaneID] = p.AgentSessionID
			}
			existingIDsByIdx[i] = p.AgentSessionID
		}
	}

	var sessionPanes []session.Pane
	agentIdx := 0
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

		if id, ok := existingIDs[p.PaneID]; ok {
			sp.AgentSessionID = id
		} else if p.IsAgent {
			if id, ok := existingIDsByIdx[agentIdx]; ok {
				sp.AgentSessionID = id
			}
		}
		if p.IsAgent {
			agentIdx++
		}

		sessionPanes = append(sessionPanes, sp)
	}

	s.Panes = sessionPanes
	session.Save(s)
}

// CleanOrphanWindows kills tmux windows that don't belong to any saved session.
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

	topbarWindow := baytmux.TopbarWindowIndex()

	for _, idx := range baytmux.ListWindowIndices() {
		if idx == 0 || idx == topbarWindow || owned[idx] {
			continue
		}
		baytmux.KillWindow(idx)
	}
}

// OnSessionDelete clears tasks for the session.
func OnSessionDelete(sessionName string) error {
	memory.ClearTasks(sessionName)
	return nil
}

// OnSessionRename updates task session_id.
func OnSessionRename(oldName, newName string) error {
	d, err := memory.GetDB()
	if err != nil {
		return nil
	}
	d.Exec(`UPDATE tasks SET session_id = ? WHERE session_id = ?`, newName, oldName)
	return nil
}
