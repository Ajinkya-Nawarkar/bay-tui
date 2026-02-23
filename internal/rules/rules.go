package rules

import (
	"database/sql"
	"fmt"
	"os"

	"bay/internal/db"
)

// Rule represents a context injection rule entry.
type Rule struct {
	Name    string
	Path    string
	Scope   string // "global" or "repo:{name}"
	Enabled bool
}

// Add registers a context file in the rules table.
func Add(name, path, scope string) error {
	return AddDB(nil, name, path, scope)
}

// AddDB registers a rule using the given DB (or default).
func AddDB(d *sql.DB, name, path, scope string) error {
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
	_, err := d.Exec(
		`INSERT INTO rules (name, path, scope, enabled) VALUES (?, ?, ?, 1)
		ON CONFLICT(name) DO UPDATE SET path = excluded.path, scope = excluded.scope`,
		name, path, scope,
	)
	return err
}

// Remove deletes a rule by name.
func Remove(name string) error {
	return RemoveDB(nil, name)
}

// RemoveDB deletes a rule using the given DB (or default).
func RemoveDB(d *sql.DB, name string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	_, err := d.Exec(`DELETE FROM rules WHERE name = ?`, name)
	return err
}

// List returns all rules.
func List() ([]Rule, error) {
	return ListDB(nil)
}

// ListDB returns all rules using the given DB (or default).
func ListDB(d *sql.DB) ([]Rule, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	rows, err := d.Query(`SELECT name, path, scope, enabled FROM rules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Name, &r.Path, &r.Scope, &r.Enabled); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// Toggle flips the enabled flag for a rule.
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
	_, err := d.Exec(`UPDATE rules SET enabled = NOT enabled WHERE name = ?`, name)
	return err
}

// ActiveRules returns enabled rules matching global + repo scope.
func ActiveRules(repoName string) ([]Rule, error) {
	return ActiveRulesDB(nil, repoName)
}

// ActiveRulesDB returns active rules using the given DB (or default).
func ActiveRulesDB(d *sql.DB, repoName string) ([]Rule, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	repoScope := "repo:" + repoName
	rows, err := d.Query(
		`SELECT name, path, scope, enabled FROM rules
		WHERE enabled = 1 AND (scope = 'global' OR scope = ?)
		ORDER BY name`, repoScope,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Name, &r.Path, &r.Scope, &r.Enabled); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// ReadContent reads the markdown content from a rule's file path.
func ReadContent(r Rule) (string, error) {
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return "", fmt.Errorf("reading rule %s at %s: %w", r.Name, r.Path, err)
	}
	return string(data), nil
}
