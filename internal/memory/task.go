package memory

import (
	"database/sql"
	"fmt"
	"time"

	"bay/internal/db"
)

// Task represents a task or subtask in a session.
type Task struct {
	ID          int64
	SessionID   string
	Title       string
	Status      string // "todo", "doing", "done"
	ParentID    *int64
	SortOrder   int
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// CreateTask inserts a new task for a session.
func CreateTask(sessionID, title string, parentID *int64) (int64, error) {
	return CreateTaskDB(nil, sessionID, title, parentID)
}

// CreateTaskDB inserts a new task using the given DB (or default).
func CreateTaskDB(d *sql.DB, sessionID, title string, parentID *int64) (int64, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return 0, fmt.Errorf("opening db: %w", err)
		}
	}

	// Determine next sort_order for this session
	var maxOrder int
	d.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) FROM tasks WHERE session_id = ?`, sessionID).Scan(&maxOrder)

	var pid sql.NullInt64
	if parentID != nil {
		pid = sql.NullInt64{Int64: *parentID, Valid: true}
	}

	res, err := d.Exec(
		`INSERT INTO tasks (session_id, title, status, parent_id, sort_order) VALUES (?, ?, 'todo', ?, ?)`,
		sessionID, title, pid, maxOrder+1,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTasks returns all tasks for a session, ordered by sort_order.
// Root tasks come first, followed by their subtasks.
func ListTasks(sessionID string) ([]Task, error) {
	return ListTasksDB(nil, sessionID)
}

// ListTasksDB returns all tasks for a session using the given DB (or default).
func ListTasksDB(d *sql.DB, sessionID string) ([]Task, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	rows, err := d.Query(
		`SELECT id, session_id, title, status, parent_id, sort_order, created_at, completed_at
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
		var parentID sql.NullInt64
		var completedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Title, &t.Status, &parentID, &t.SortOrder, &t.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		if parentID.Valid {
			pid := parentID.Int64
			t.ParentID = &pid
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// SetTaskStatus updates a task's status. Sets completed_at when status is "done".
func SetTaskStatus(id int64, status string) error {
	return SetTaskStatusDB(nil, id, status)
}

// SetTaskStatusDB updates a task's status using the given DB (or default).
func SetTaskStatusDB(d *sql.DB, id int64, status string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	if status == "done" {
		_, err := d.Exec(`UPDATE tasks SET status = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
		return err
	}
	_, err := d.Exec(`UPDATE tasks SET status = ?, completed_at = NULL WHERE id = ?`, status, id)
	return err
}

// DeleteTask removes a task and its subtasks.
func DeleteTask(id int64) error {
	return DeleteTaskDB(nil, id)
}

// DeleteTaskDB removes a task and its subtasks using the given DB (or default).
func DeleteTaskDB(d *sql.DB, id int64) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}

	// Delete subtasks first
	if _, err := d.Exec(`DELETE FROM tasks WHERE parent_id = ?`, id); err != nil {
		return err
	}
	_, err := d.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// ClearTasks removes all tasks for a session.
func ClearTasks(sessionID string) error {
	return ClearTasksDB(nil, sessionID)
}

// ClearTasksDB removes all tasks for a session using the given DB (or default).
func ClearTasksDB(d *sql.DB, sessionID string) error {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return fmt.Errorf("opening db: %w", err)
		}
	}
	_, err := d.Exec(`DELETE FROM tasks WHERE session_id = ?`, sessionID)
	return err
}

// GetTaskByID returns a single task by its DB ID.
func GetTaskByID(id int64) (*Task, error) {
	return GetTaskByIDDB(nil, id)
}

// GetTaskByIDDB returns a single task by its DB ID using the given DB.
func GetTaskByIDDB(d *sql.DB, id int64) (*Task, error) {
	if d == nil {
		var err error
		d, err = db.Open()
		if err != nil {
			return nil, fmt.Errorf("opening db: %w", err)
		}
	}

	var t Task
	var parentID sql.NullInt64
	var completedAt sql.NullTime
	err := d.QueryRow(
		`SELECT id, session_id, title, status, parent_id, sort_order, created_at, completed_at
		FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.SessionID, &t.Title, &t.Status, &parentID, &t.SortOrder, &t.CreatedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		pid := parentID.Int64
		t.ParentID = &pid
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}

// ResolveDisplayID maps a 1-based display index to a Task from the ordered task list.
// Display IDs follow this numbering: root tasks get sequential numbers (1, 2, 3...),
// subtasks get parent.child notation (1.1, 1.2, 2.1...) but for resolving by integer,
// we use a flat sequential index.
func ResolveDisplayID(tasks []Task, displayID int) *Task {
	if displayID < 1 || displayID > len(tasks) {
		return nil
	}
	return &tasks[displayID-1]
}
