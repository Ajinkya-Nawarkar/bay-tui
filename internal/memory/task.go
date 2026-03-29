package memory

import (
	"database/sql"
	"time"
)

// Task represents a checklist item in a session.
type Task struct {
	ID          int64
	SessionID   string
	Title       string
	Status      string // "todo", "done"
	SortOrder   int
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// CreateTask inserts a new checklist item for a session.
func CreateTask(sessionID, title string) (int64, error) {
	return CreateTaskDB(nil, sessionID, title)
}

// CreateTaskDB inserts a new checklist item using the given DB (or default).
func CreateTaskDB(d *sql.DB, sessionID, title string) (int64, error) {
	var err error
	if d, err = ensureDB(d); err != nil {
		return 0, err
	}

	var maxOrder int
	if err := d.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) FROM tasks WHERE session_id = ?`, sessionID).Scan(&maxOrder); err != nil {
		maxOrder = 0
	}

	res, err := d.Exec(
		`INSERT INTO tasks (session_id, title, status, sort_order) VALUES (?, ?, 'todo', ?)`,
		sessionID, title, maxOrder+1,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTasks returns all checklist items for a session, ordered by sort_order.
func ListTasks(sessionID string) ([]Task, error) {
	return ListTasksDB(nil, sessionID)
}

// ListTasksDB returns all checklist items using the given DB (or default).
func ListTasksDB(d *sql.DB, sessionID string) ([]Task, error) {
	var err error
	if d, err = ensureDB(d); err != nil {
		return nil, err
	}

	rows, err := d.Query(
		`SELECT id, session_id, title, status, sort_order, created_at, completed_at
		FROM tasks WHERE session_id = ? ORDER BY sort_order`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Title, &t.Status, &t.SortOrder, &t.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// SetTaskStatus updates a checklist item's status ("todo" or "done").
func SetTaskStatus(id int64, status string) error {
	return SetTaskStatusDB(nil, id, status)
}

// SetTaskStatusDB updates a checklist item's status using the given DB.
func SetTaskStatusDB(d *sql.DB, id int64, status string) error {
	var err error
	if d, err = ensureDB(d); err != nil {
		return err
	}

	if status == "done" {
		_, err = d.Exec(`UPDATE tasks SET status = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
		return err
	}
	_, err = d.Exec(`UPDATE tasks SET status = ?, completed_at = NULL WHERE id = ?`, status, id)
	return err
}

// DeleteTask removes a checklist item.
func DeleteTask(id int64) error {
	return DeleteTaskDB(nil, id)
}

// DeleteTaskDB removes a checklist item using the given DB.
func DeleteTaskDB(d *sql.DB, id int64) error {
	var err error
	if d, err = ensureDB(d); err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// ClearTasks removes all checklist items for a session.
func ClearTasks(sessionID string) error {
	return ClearTasksDB(nil, sessionID)
}

// ClearTasksDB removes all checklist items using the given DB.
func ClearTasksDB(d *sql.DB, sessionID string) error {
	var err error
	if d, err = ensureDB(d); err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM tasks WHERE session_id = ?`, sessionID)
	return err
}

// GetTaskByID returns a single checklist item by its DB ID.
func GetTaskByID(id int64) (*Task, error) {
	return GetTaskByIDDB(nil, id)
}

// GetTaskByIDDB returns a single checklist item using the given DB.
func GetTaskByIDDB(d *sql.DB, id int64) (*Task, error) {
	var err error
	if d, err = ensureDB(d); err != nil {
		return nil, err
	}

	var t Task
	var completedAt sql.NullTime
	err = d.QueryRow(
		`SELECT id, session_id, title, status, sort_order, created_at, completed_at
		FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.SessionID, &t.Title, &t.Status, &t.SortOrder, &t.CreatedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}

// ResolveDisplayID maps a 1-based display index to a Task from the ordered list.
func ResolveDisplayID(tasks []Task, displayID int) *Task {
	if displayID < 1 || displayID > len(tasks) {
		return nil
	}
	return &tasks[displayID-1]
}
