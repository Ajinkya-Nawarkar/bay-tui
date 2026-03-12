package context

import (
	"database/sql"
	"fmt"
	"os"

	"bay/internal/db"
)

// ContextFile represents a registered context file entry.
type ContextFile struct {
	Name        string
	Path        string
	Scope       string // "global" or "repo:{name}"
	Enabled     bool
	Category    string // "rules", "docs", "standards", etc.
	Type        string // "rules", "skills", "agents", "plugins" — maps to ~/.bay/{type}/ subdir
	Description string
}

// Add registers a context file in the context_files table.
func Add(name, path, scope, category, typ, description string) error {
	return AddDB(nil, name, path, scope, category, typ, description)
}

// AddDB registers a context file using the given DB (or default).
func AddDB(d *sql.DB, name, path, scope, category, typ, description string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	if scope == "" {
		scope = "global"
	}
	if category == "" {
		category = "rules"
	}
	if typ == "" {
		typ = "rules"
	}
	_, err := d.Exec(
		`INSERT INTO context_files (name, path, scope, enabled, category, type, description) VALUES (?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET path = excluded.path, scope = excluded.scope, category = excluded.category, type = excluded.type, description = excluded.description`,
		name, path, scope, category, typ, description,
	)
	return err
}

// Remove deletes a context file by name.
func Remove(name string) error {
	return RemoveDB(nil, name)
}

// RemoveDB deletes a context file using the given DB (or default).
func RemoveDB(d *sql.DB, name string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	_, err := d.Exec(`DELETE FROM context_files WHERE name = ?`, name)
	return err
}

// List returns all context files.
func List() ([]ContextFile, error) {
	return ListDB(nil)
}

// ListDB returns all context files using the given DB (or default).
func ListDB(d *sql.DB) ([]ContextFile, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	rows, err := d.Query(`SELECT name, path, scope, enabled, category, COALESCE(type, 'rules'), COALESCE(description, '') FROM context_files ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []ContextFile
	for rows.Next() {
		var f ContextFile
		if err := rows.Scan(&f.Name, &f.Path, &f.Scope, &f.Enabled, &f.Category, &f.Type, &f.Description); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// Toggle flips the enabled flag for a context file.
func Toggle(name string) error {
	return ToggleDB(nil, name)
}

// ToggleDB flips the enabled flag using the given DB (or default).
func ToggleDB(d *sql.DB, name string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	_, err := d.Exec(`UPDATE context_files SET enabled = NOT enabled WHERE name = ?`, name)
	return err
}

// ActiveRules returns enabled context files matching global + repo scope.
func ActiveRules(repoName string) ([]ContextFile, error) {
	return ActiveRulesDB(nil, repoName)
}

// ActiveRulesDB returns active context files using the given DB (or default).
func ActiveRulesDB(d *sql.DB, repoName string) ([]ContextFile, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	repoScope := "repo:" + repoName
	rows, err := d.Query(
		`SELECT name, path, scope, enabled, category, COALESCE(type, 'rules'), COALESCE(description, '') FROM context_files
		WHERE enabled = 1 AND (scope = 'global' OR scope = ?)
		ORDER BY name`, repoScope,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []ContextFile
	for rows.Next() {
		var f ContextFile
		if err := rows.Scan(&f.Name, &f.Path, &f.Scope, &f.Enabled, &f.Category, &f.Type, &f.Description); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ReadContent reads the markdown content from a context file's path.
func ReadContent(f ContextFile) (string, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return "", fmt.Errorf("reading context file %s at %s: %w", f.Name, f.Path, err)
	}
	return string(data), nil
}
