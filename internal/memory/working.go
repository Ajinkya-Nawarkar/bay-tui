package memory

import (
	"database/sql"
	"fmt"
	"time"

	"bay/internal/db"
)

// GetWorking loads the working state for a session. Returns nil if not found.
func GetWorking(sessionID string) (*WorkingState, error) {
	return GetWorkingDB(nil, sessionID)
}

// GetWorkingDB loads working state using the given DB (or default).
func GetWorkingDB(d *sql.DB, sessionID string) (*WorkingState, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	w := &WorkingState{}
	var activeSince sql.NullTime
	var worktreePath, gitBranch, claudeSessionID, currentTask, lastSummary sql.NullString

	err := d.QueryRow(
		`SELECT session_id, repo, worktree_path, git_branch, claude_session_id,
			current_task, last_summary, active_since, last_updated
		FROM working_state WHERE session_id = ?`, sessionID,
	).Scan(
		&w.SessionID, &w.Repo, &worktreePath, &gitBranch, &claudeSessionID,
		&currentTask, &lastSummary, &activeSince, &w.LastUpdated,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	w.WorktreePath = worktreePath.String
	w.GitBranch = gitBranch.String
	w.ClaudeSessionID = claudeSessionID.String
	w.CurrentTask = currentTask.String
	w.LastSummary = lastSummary.String
	if activeSince.Valid {
		w.ActiveSince = &activeSince.Time
	}

	return w, nil
}

// UpsertWorking creates or updates the working state.
func UpsertWorking(w *WorkingState) error {
	return UpsertWorkingDB(nil, w)
}

// UpsertWorkingDB creates or updates working state using the given DB (or default).
func UpsertWorkingDB(d *sql.DB, w *WorkingState) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	_, err := d.Exec(
		`INSERT INTO working_state (session_id, repo, worktree_path, git_branch,
			claude_session_id, current_task, last_summary, active_since, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			repo = excluded.repo,
			worktree_path = excluded.worktree_path,
			git_branch = excluded.git_branch,
			claude_session_id = excluded.claude_session_id,
			current_task = excluded.current_task,
			last_summary = excluded.last_summary,
			active_since = excluded.active_since,
			last_updated = excluded.last_updated`,
		w.SessionID, w.Repo, nullStr(w.WorktreePath), nullStr(w.GitBranch),
		nullStr(w.ClaudeSessionID), nullStr(w.CurrentTask), nullStr(w.LastSummary),
		w.ActiveSince, time.Now(),
	)
	return err
}

// SetTask updates just the current_task field.
func SetTask(sessionID, task string) error {
	return SetTaskDB(nil, sessionID, task)
}

// SetTaskDB updates current_task using the given DB (or default).
func SetTaskDB(d *sql.DB, sessionID, task string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	_, err := d.Exec(
		`UPDATE working_state SET current_task = ?, last_updated = CURRENT_TIMESTAMP WHERE session_id = ?`,
		task, sessionID,
	)
	return err
}

// SetSummary updates the last_summary field.
func SetSummary(sessionID, summary string) error {
	return SetSummaryDB(nil, sessionID, summary)
}

// SetSummaryDB updates last_summary using the given DB (or default).
func SetSummaryDB(d *sql.DB, sessionID, summary string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	_, err := d.Exec(
		`UPDATE working_state SET last_summary = ?, last_updated = CURRENT_TIMESTAMP WHERE session_id = ?`,
		summary, sessionID,
	)
	return err
}

// DeleteWorking removes the working state for a session.
func DeleteWorking(sessionID string) error {
	return DeleteWorkingDB(nil, sessionID)
}

// DeleteWorkingDB removes working state using the given DB (or default).
func DeleteWorkingDB(d *sql.DB, sessionID string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	_, err := d.Exec(`DELETE FROM working_state WHERE session_id = ?`, sessionID)
	return err
}

// RenameWorking updates the session_id in working_state.
func RenameWorking(oldID, newID string) error {
	return RenameWorkingDB(nil, oldID, newID)
}

// RenameWorkingDB renames session_id using the given DB (or default).
func RenameWorkingDB(d *sql.DB, oldID, newID string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	_, err := d.Exec(`UPDATE working_state SET session_id = ? WHERE session_id = ?`, newID, oldID)
	return err
}

// nullStr returns a sql.NullString for optional string fields.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
