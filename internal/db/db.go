package db

import (
	"database/sql"
	"sync"

	"bay/internal/config"
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
		// Enable WAL mode for better concurrent read/write performance.
		conn.Exec("PRAGMA journal_mode=WAL")
		conn.Exec("PRAGMA busy_timeout=5000")
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
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// DBPath returns ~/.bay/bay.db
func DBPath() string {
	return config.BayDir() + "/bay.db"
}

// migrate runs CREATE TABLE IF NOT EXISTS for all tables + FTS + triggers.
func migrate(db *sql.DB) error {
	stmts := []string{
		// Episodic Memory: raw event log
		`CREATE TABLE IF NOT EXISTS episodic (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id  TEXT NOT NULL,
			type        TEXT NOT NULL,
			content     TEXT NOT NULL,
			pane_id     TEXT,
			timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// FTS5 virtual table for full-text search over episodic content
		`CREATE VIRTUAL TABLE IF NOT EXISTS episodic_fts USING fts5(
			content,
			session_id,
			type,
			content='episodic',
			content_rowid='id'
		)`,

		// Working Memory: live session state
		`CREATE TABLE IF NOT EXISTS working_state (
			session_id        TEXT PRIMARY KEY,
			repo              TEXT NOT NULL,
			worktree_path     TEXT,
			git_branch        TEXT,
			claude_session_id TEXT,
			current_task      TEXT,
			last_summary      TEXT,
			active_since      DATETIME,
			last_updated      DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Procedural Memory: context injection rules
		`CREATE TABLE IF NOT EXISTS rules (
			name    TEXT PRIMARY KEY,
			path    TEXT NOT NULL,
			scope   TEXT DEFAULT 'global',
			enabled BOOLEAN DEFAULT 1
		)`,

		// Pending summaries: raw buffers awaiting async LLM summarization
		`CREATE TABLE IF NOT EXISTS pending_summaries (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id  TEXT NOT NULL,
			raw_buffer  TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	// Create triggers only if they don't exist.
	// SQLite doesn't have CREATE TRIGGER IF NOT EXISTS, so we check first.
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name='episodic_ai'").Scan(&count)
	if count == 0 {
		_, err := db.Exec(`CREATE TRIGGER episodic_ai AFTER INSERT ON episodic BEGIN
			INSERT INTO episodic_fts(rowid, content, session_id, type)
			VALUES (new.id, new.content, new.session_id, new.type);
		END`)
		if err != nil {
			return err
		}
	}

	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name='episodic_ad'").Scan(&count)
	if count == 0 {
		_, err := db.Exec(`CREATE TRIGGER episodic_ad AFTER DELETE ON episodic BEGIN
			INSERT INTO episodic_fts(episodic_fts, rowid, content, session_id, type)
			VALUES ('delete', old.id, old.content, old.session_id, old.type);
		END`)
		if err != nil {
			return err
		}
	}

	// Add claude_session_id column to episodic (try-and-ignore for existing DBs)
	db.Exec(`ALTER TABLE episodic ADD COLUMN claude_session_id TEXT`)

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
