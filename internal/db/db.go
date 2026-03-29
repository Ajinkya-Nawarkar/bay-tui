// Package db provides a singleton SQLite connection to ~/.bay/bay.db.
package db

import (
	"database/sql"
	"path/filepath"
	"sync"

	"bay/internal/config"
	"bay/internal/logging"
	_ "modernc.org/sqlite"
)

var (
	once sync.Once
	conn *sql.DB
	err  error
)

// Open returns the singleton DB connection. Creates + migrates on first call.
func Open() (*sql.DB, error) {
	once.Do(func() {
		path := DBPath()
		conn, err = sql.Open("sqlite", path)
		if err != nil {
			return
		}
		if _, e := conn.Exec("PRAGMA journal_mode=WAL"); e != nil {
			logging.Warn("PRAGMA journal_mode=WAL failed: %v", e)
		}
		if _, e := conn.Exec("PRAGMA busy_timeout=5000"); e != nil {
			logging.Warn("PRAGMA busy_timeout failed: %v", e)
		}
		err = migrate(conn)
	})
	return conn, err
}

// OpenPath opens a specific database path (for testing with :memory:).
func OpenPath(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, e := db.Exec("PRAGMA journal_mode=WAL"); e != nil {
		logging.Warn("PRAGMA journal_mode=WAL failed: %v", e)
	}
	if _, e := db.Exec("PRAGMA busy_timeout=5000"); e != nil {
		logging.Warn("PRAGMA busy_timeout failed: %v", e)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// DBPath returns ~/.bay/bay.db
func DBPath() string {
	return filepath.Join(config.BayDir(), "bay.db")
}

// migrate runs CREATE TABLE IF NOT EXISTS for all tables.
func migrate(db *sql.DB) error {
	stmts := []string{
		// Tasks: flat checklist per session
		`CREATE TABLE IF NOT EXISTS tasks (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id   TEXT NOT NULL,
			title        TEXT NOT NULL,
			status       TEXT DEFAULT 'todo',
			parent_id    INTEGER,
			sort_order   INTEGER DEFAULT 0,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,

		`CREATE INDEX IF NOT EXISTS idx_tasks_session ON tasks(session_id)`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	return nil
}

// ResetSingleton clears the singleton connection (for testing).
func ResetSingleton() {
	if conn != nil {
		conn.Close()
	}
	once = sync.Once{}
	conn = nil
	err = nil
}
