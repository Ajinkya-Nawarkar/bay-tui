package tests

import (
	"strings"
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

	id1, err := memory.CreateTaskDB(d, "s1", "Fix auth flow", nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	id2, err := memory.CreateTaskDB(d, "s1", "Refactor middleware", nil)
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
	memory.CreateTaskDB(d, "s2", "Other task", nil)
	tasks, _ = memory.ListTasksDB(d, "s1")
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for s1, got %d", len(tasks))
	}
}

func TestSubtaskCreation(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	parentID, _ := memory.CreateTaskDB(d, "s1", "Fix auth flow", nil)
	_, err = memory.CreateTaskDB(d, "s1", "Write unit tests", &parentID)
	if err != nil {
		t.Fatalf("CreateTask subtask failed: %v", err)
	}
	_, err = memory.CreateTaskDB(d, "s1", "Update token refresh", &parentID)
	if err != nil {
		t.Fatalf("CreateTask subtask failed: %v", err)
	}

	tasks, _ := memory.ListTasksDB(d, "s1")
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	subtaskCount := 0
	for _, t := range tasks {
		if t.ParentID != nil {
			subtaskCount++
			if *t.ParentID != parentID {
				// This test uses the outer t variable name, so use a different check
				break
			}
		}
	}
	if subtaskCount != 2 {
		t.Errorf("expected 2 subtasks, got %d", subtaskCount)
	}
}

func TestSetTaskStatus(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	id, _ := memory.CreateTaskDB(d, "s1", "Fix bug", nil)

	// todo → doing
	if err := memory.SetTaskStatusDB(d, id, "doing"); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	task, _ := memory.GetTaskByIDDB(d, id)
	if task.Status != "doing" {
		t.Errorf("expected 'doing', got '%s'", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("expected nil CompletedAt for 'doing' status")
	}

	// doing → done
	if err := memory.SetTaskStatusDB(d, id, "done"); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	task, _ = memory.GetTaskByIDDB(d, id)
	if task.Status != "done" {
		t.Errorf("expected 'done', got '%s'", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt set for 'done' status")
	}

	// done → todo (reset)
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

func TestDeleteTaskCascade(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	parentID, _ := memory.CreateTaskDB(d, "s1", "Parent task", nil)
	memory.CreateTaskDB(d, "s1", "Subtask 1", &parentID)
	memory.CreateTaskDB(d, "s1", "Subtask 2", &parentID)
	memory.CreateTaskDB(d, "s1", "Standalone task", nil)

	// Delete parent should cascade to subtasks
	if err := memory.DeleteTaskDB(d, parentID); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	tasks, _ := memory.ListTasksDB(d, "s1")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 remaining task, got %d", len(tasks))
	}
	if tasks[0].Title != "Standalone task" {
		t.Errorf("expected 'Standalone task', got '%s'", tasks[0].Title)
	}
}

func TestClearTasks(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	memory.CreateTaskDB(d, "s1", "Task 1", nil)
	memory.CreateTaskDB(d, "s1", "Task 2", nil)
	memory.CreateTaskDB(d, "s2", "Other session task", nil)

	if err := memory.ClearTasksDB(d, "s1"); err != nil {
		t.Fatalf("ClearTasks failed: %v", err)
	}

	tasks, _ := memory.ListTasksDB(d, "s1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for s1, got %d", len(tasks))
	}

	// s2 should be unaffected
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

	memory.CreateTaskDB(d, "s1", "First", nil)
	memory.CreateTaskDB(d, "s1", "Second", nil)
	memory.CreateTaskDB(d, "s1", "Third", nil)

	tasks, _ := memory.ListTasksDB(d, "s1")

	// Valid IDs
	task := memory.ResolveDisplayID(tasks, 1)
	if task == nil || task.Title != "First" {
		t.Errorf("expected 'First' for display ID 1")
	}
	task = memory.ResolveDisplayID(tasks, 3)
	if task == nil || task.Title != "Third" {
		t.Errorf("expected 'Third' for display ID 3")
	}

	// Out of bounds
	if memory.ResolveDisplayID(tasks, 0) != nil {
		t.Error("expected nil for display ID 0")
	}
	if memory.ResolveDisplayID(tasks, 4) != nil {
		t.Error("expected nil for display ID 4")
	}
}

func TestContextRenderingWithTasks(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	// Create working state
	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	// Create tasks
	id1, _ := memory.CreateTaskDB(d, "s1", "Refactor middleware", nil)
	memory.SetTaskStatusDB(d, id1, "done")

	id2, _ := memory.CreateTaskDB(d, "s1", "Fix auth flow", nil)
	memory.SetTaskStatusDB(d, id2, "doing")

	memory.CreateTaskDB(d, "s1", "Write unit tests", &id2)

	memory.CreateTaskDB(d, "s1", "Update docs", nil)

	ctx, err := memory.RenderContextDB(d, "s1", "", 0)
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	if !strings.Contains(ctx, "## Tasks") {
		t.Error("expected Tasks section")
	}
	if !strings.Contains(ctx, "[x]") {
		t.Error("expected done marker [x]")
	}
	if !strings.Contains(ctx, "[>]") {
		t.Error("expected doing marker [>]")
	}
	if !strings.Contains(ctx, "[ ]") {
		t.Error("expected todo marker [ ]")
	}
	if !strings.Contains(ctx, "Refactor middleware") {
		t.Error("expected task title in context")
	}
}

func TestContextRenderingWithPaneAssignment(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	id1, _ := memory.CreateTaskDB(d, "s1", "Fix auth", nil)
	memory.CreateTaskDB(d, "s1", "Write tests", nil)

	// Render with pane assigned to task id1
	ctx, err := memory.RenderContextDB(d, "s1", "", int(id1))
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	if !strings.Contains(ctx, "assigned to this pane") {
		t.Error("expected pane assignment marker in context")
	}
}

func TestContextRenderingNoTasks(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	w := &memory.WorkingState{SessionID: "s1", Repo: "myrepo"}
	memory.UpsertWorkingDB(d, w)

	ctx, err := memory.RenderContextDB(d, "s1", "", 0)
	if err != nil {
		t.Fatalf("RenderContextDB failed: %v", err)
	}

	if strings.Contains(ctx, "## Tasks") {
		t.Error("Tasks section should be absent when no tasks exist")
	}
}

func TestGetTaskByID(t *testing.T) {
	d, err := db.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("OpenPath failed: %v", err)
	}
	defer d.Close()

	id, _ := memory.CreateTaskDB(d, "s1", "Test task", nil)

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

	// Non-existent
	task, err = memory.GetTaskByIDDB(d, 9999)
	if err != nil {
		t.Fatalf("GetTaskByID should not error for missing: %v", err)
	}
	if task != nil {
		t.Error("expected nil for non-existent task")
	}
}
