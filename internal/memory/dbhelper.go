package memory

import (
	"database/sql"
	"fmt"

	"bay/internal/db"
)

// ensureDB returns d if non-nil, otherwise opens the default database.
// This eliminates the repeated open-if-nil pattern across the memory package.
func ensureDB(d *sql.DB) (*sql.DB, error) {
	if d != nil {
		return d, nil
	}
	conn, err := db.Open()
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	return conn, nil
}

// GetDB returns the singleton database connection.
// Exported for use by hooks and other packages that need direct DB access.
func GetDB() (*sql.DB, error) {
	return db.Open()
}
