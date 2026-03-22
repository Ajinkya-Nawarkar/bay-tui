// Package memory implements bay's three-layer memory system: episodic, working, and tasks.
//
// Episodic: append-only event log with FTS5 full-text search.
// Working: mutable per-session state (repo, branch, summary, current task).
// Tasks: hierarchical task tracking with parent/child relationships.
// Summarize: async LLM summarization pipeline with retry + compaction.
//
// All *DB functions accept an optional *sql.DB — pass nil to use the singleton.
package memory

import (
	"database/sql"
)

// AppendEpisodic inserts a raw event into the episodic table.
func AppendEpisodic(sessionID, eventType, content, paneID string) error {
	return AppendEpisodicDB(nil, sessionID, eventType, content, paneID)
}

// AppendEpisodicDB inserts a raw event using the given DB (or default).
func AppendEpisodicDB(d *sql.DB, sessionID, eventType, content, paneID string) error {
	var err error
	if d, err = ensureDB(d); err != nil {
		return err
	}
	_, err = d.Exec(
		`INSERT INTO episodic (session_id, type, content, pane_id) VALUES (?, ?, ?, ?)`,
		sessionID, eventType, content, nullStr(paneID),
	)
	return err
}

// RecentEpisodic returns the last N entries for a session.
func RecentEpisodic(sessionID string, n int) ([]EpisodicEntry, error) {
	return RecentEpisodicDB(nil, sessionID, n)
}

// RecentEpisodicDB returns recent entries using the given DB (or default).
func RecentEpisodicDB(d *sql.DB, sessionID string, n int) ([]EpisodicEntry, error) {
	var err error
	if d, err = ensureDB(d); err != nil {
		return nil, err
	}

	query := `SELECT id, session_id, type, content, COALESCE(pane_id, ''), timestamp
		FROM episodic WHERE session_id = ? ORDER BY id DESC LIMIT ?`
	rows, err := d.Query(query, sessionID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EpisodicEntry
	for rows.Next() {
		var e EpisodicEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Content, &e.PaneID, &e.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// RecentSummariesDB returns the last N summary entries for a session, newest first.
func RecentSummariesDB(d *sql.DB, sessionID string, n int) ([]EpisodicEntry, error) {
	var err error
	if d, err = ensureDB(d); err != nil {
		return nil, err
	}

	query := `SELECT id, session_id, type, content, COALESCE(pane_id, ''), timestamp
		FROM episodic WHERE session_id = ? AND type = 'summary' ORDER BY id DESC LIMIT ?`
	rows, err := d.Query(query, sessionID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EpisodicEntry
	for rows.Next() {
		var e EpisodicEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Content, &e.PaneID, &e.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SearchEpisodic runs FTS5 query across all sessions (or filtered to one).
func SearchEpisodic(query string, sessionID string) ([]EpisodicEntry, error) {
	return SearchEpisodicDB(nil, query, sessionID)
}

// SearchEpisodicDB runs FTS5 query using the given DB (or default).
func SearchEpisodicDB(d *sql.DB, query string, sessionID string) ([]EpisodicEntry, error) {
	var err error
	if d, err = ensureDB(d); err != nil {
		return nil, err
	}

	var sqlQuery string
	var args []any

	if sessionID != "" {
		sqlQuery = `SELECT e.id, e.session_id, e.type, e.content, COALESCE(e.pane_id, ''), e.timestamp
			FROM episodic e
			JOIN episodic_fts f ON e.id = f.rowid
			WHERE episodic_fts MATCH ? AND e.session_id = ?
			ORDER BY e.id DESC LIMIT 50`
		args = []any{query, sessionID}
	} else {
		sqlQuery = `SELECT e.id, e.session_id, e.type, e.content, COALESCE(e.pane_id, ''), e.timestamp
			FROM episodic e
			JOIN episodic_fts f ON e.id = f.rowid
			WHERE episodic_fts MATCH ?
			ORDER BY e.id DESC LIMIT 50`
		args = []any{query}
	}

	rows, err := d.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EpisodicEntry
	for rows.Next() {
		var e EpisodicEntry
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Content, &e.PaneID, &e.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DeleteSessionEpisodic deletes all episodic entries for a session.
func DeleteSessionEpisodic(sessionID string) error {
	return DeleteSessionEpisodicDB(nil, sessionID)
}

// DeleteSessionEpisodicDB deletes all episodic entries using the given DB (or default).
func DeleteSessionEpisodicDB(d *sql.DB, sessionID string) error {
	var err error
	if d, err = ensureDB(d); err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM episodic WHERE session_id = ?`, sessionID)
	return err
}

// nullStr returns a sql.NullString for optional string fields.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
