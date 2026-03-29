package tests

import (
	"testing"

	"bay/internal/db"
	"bay/internal/memory"
)

func TestCreateAndListTasks(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	id1, err := memory.CreateTaskDB(d, "s1", "Fix auth flow")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	id2, err := memory.CreateTaskDB(d, "s1", "Refactor middleware")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if id1 == 0 || id2 == 0 {
		t.Error("expected non-zero IDs")
	}

	tasks, err := memory.ListTasksDB(d, "s1")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Title != "Fix auth flow" {
		t.Errorf("expected first task 'Fix auth flow', got '%s'", tasks[0].Title)
	}
	if tasks[0].Status != "todo" {
		t.Errorf("expected default status 'todo', got '%s'", tasks[0].Status)
	}

	// Separate sessions
	memory.CreateTaskDB(d, "s2", "Other task")
	tasks, _ = memory.ListTasksDB(d, "s1")
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for s1, got %d", len(tasks))
	}
}

func TestSetTaskStatus(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	id, _ := memory.CreateTaskDB(d, "s1", "Fix bug")

	// todo → done
	if err := memory.SetTaskStatusDB(d, id, "done"); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	task, _ := memory.GetTaskByIDDB(d, id)
	if task.Status != "done" {
		t.Errorf("expected 'done', got '%s'", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt set for 'done' status")
	}

	// done → todo (undo)
	if err := memory.SetTaskStatusDB(d, id, "todo"); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	task, _ = memory.GetTaskByIDDB(d, id)
	if task.Status != "todo" {
		t.Errorf("expected 'todo', got '%s'", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("expected nil CompletedAt after reset to 'todo'")
	}
}

func TestDeleteTask(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	id1, _ := memory.CreateTaskDB(d, "s1", "Task 1")
	memory.CreateTaskDB(d, "s1", "Task 2")

	if err := memory.DeleteTaskDB(d, id1); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	tasks, _ := memory.ListTasksDB(d, "s1")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 remaining task, got %d", len(tasks))
	}
	if tasks[0].Title != "Task 2" {
		t.Errorf("expected 'Task 2', got '%s'", tasks[0].Title)
	}
}

func TestClearTasks(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	memory.CreateTaskDB(d, "s1", "Task 1")
	memory.CreateTaskDB(d, "s1", "Task 2")
	memory.CreateTaskDB(d, "s2", "Other session task")

	if err := memory.ClearTasksDB(d, "s1"); err != nil {
		t.Fatalf("ClearTasks failed: %v", err)
	}

	tasks, _ := memory.ListTasksDB(d, "s1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for s1, got %d", len(tasks))
	}

	tasks, _ = memory.ListTasksDB(d, "s2")
	if len(tasks) != 1 {
		t.Errorf("expected 1 task for s2, got %d", len(tasks))
	}
}

func TestResolveDisplayID(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	memory.CreateTaskDB(d, "s1", "First")
	memory.CreateTaskDB(d, "s1", "Second")
	memory.CreateTaskDB(d, "s1", "Third")

	tasks, _ := memory.ListTasksDB(d, "s1")

	task := memory.ResolveDisplayID(tasks, 1)
	if task == nil || task.Title != "First" {
		t.Errorf("expected 'First' for display ID 1")
	}
	task = memory.ResolveDisplayID(tasks, 3)
	if task == nil || task.Title != "Third" {
		t.Errorf("expected 'Third' for display ID 3")
	}

	if memory.ResolveDisplayID(tasks, 0) != nil {
		t.Error("expected nil for display ID 0")
	}
	if memory.ResolveDisplayID(tasks, 4) != nil {
		t.Error("expected nil for display ID 4")
	}
}

func TestGetTaskByID(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	id, _ := memory.CreateTaskDB(d, "s1", "Test task")

	task, err := memory.GetTaskByIDDB(d, id)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.Title != "Test task" {
		t.Errorf("expected 'Test task', got '%s'", task.Title)
	}

	task, err = memory.GetTaskByIDDB(d, 9999)
	if err != nil {
		t.Fatalf("GetTaskByID should not error for missing: %v", err)
	}
	if task != nil {
		t.Error("expected nil for non-existent task")
	}
}
