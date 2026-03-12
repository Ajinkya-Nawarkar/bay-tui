package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"bay/internal/memory"
	"bay/internal/session"
)

// Task handles the `bay task` subcommands.
func Task(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `Usage:
  bay task "description"       Create a task
  bay task add "desc" [-p ID]  Add a subtask
  bay task ls                  List tasks
  bay task done <id>           Mark done
  bay task doing <id>          Mark in-progress
  bay task rm <id>             Remove task
  bay task assign <id>         Assign current pane to task
  bay task clear               Clear all tasks`)
		return nil
	}

	switch args[0] {
	case "add":
		return taskAdd(args[1:])
	case "ls", "list":
		return taskList()
	case "done":
		return taskSetStatus(args[1:], "done")
	case "doing":
		return taskSetStatus(args[1:], "doing")
	case "rm", "remove":
		return taskRemove(args[1:])
	case "assign":
		return taskAssign(args[1:])
	case "clear":
		return taskClear()
	case "help", "--help", "-h":
		return Task(nil)
	default:
		// Treat as: bay task "description" — create a root task
		return taskCreate(strings.Join(args, " "), nil)
	}
}

func activeSession() (*session.Session, error) {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil, fmt.Errorf("no active session: %w", err)
	}
	return s, nil
}

func taskCreate(title string, parentID *int64) error {
	s, err := activeSession()
	if err != nil {
		return err
	}

	id, err := memory.CreateTask(s.Name, title, parentID)
	if err != nil {
		return fmt.Errorf("creating task: %w", err)
	}

	tasks, _ := memory.ListTasks(s.Name)
	displayID := 0
	for i, t := range tasks {
		if t.ID == id {
			displayID = i + 1
			break
		}
	}

	fmt.Printf("Created task #%d: %s\n", displayID, title)
	return nil
}

func taskAdd(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `Usage: bay task add "description" [-p parent_id]`)
		return nil
	}

	var parentDisplayID int
	var titleParts []string

	for i := 0; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			parentDisplayID, _ = strconv.Atoi(args[i+1])
			i++
		} else {
			titleParts = append(titleParts, args[i])
		}
	}

	title := strings.Join(titleParts, " ")
	if title == "" {
		fmt.Fprintln(os.Stderr, "Usage: bay task add \"description\" [-p parent_id]")
		return nil
	}

	var parentID *int64
	if parentDisplayID > 0 {
		s, err := activeSession()
		if err != nil {
			return err
		}
		tasks, err := memory.ListTasks(s.Name)
		if err != nil {
			return err
		}
		parent := memory.ResolveDisplayID(tasks, parentDisplayID)
		if parent == nil {
			return fmt.Errorf("task #%d not found", parentDisplayID)
		}
		parentID = &parent.ID
	}

	return taskCreate(title, parentID)
}

func taskList() error {
	s, err := activeSession()
	if err != nil {
		return err
	}

	tasks, err := memory.ListTasks(s.Name)
	if err != nil {
		return fmt.Errorf("listing tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks.")
		return nil
	}

	// Build a map of DB ID → display ID for parent lookup
	idToDisplay := make(map[int64]int)
	for i, t := range tasks {
		idToDisplay[t.ID] = i + 1
	}

	for i, t := range tasks {
		marker := statusMarker(t.Status)
		prefix := ""
		if t.ParentID != nil {
			prefix = "  "
		}
		fmt.Printf("%s%s %d. %s\n", prefix, marker, i+1, t.Title)
	}

	return nil
}

func statusMarker(status string) string {
	switch status {
	case "done":
		return "[x]"
	case "doing":
		return "[>]"
	default:
		return "[ ]"
	}
}

func taskSetStatus(args []string, status string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: bay task %s <id>\n", status)
		return nil
	}

	displayID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	s, err := activeSession()
	if err != nil {
		return err
	}

	tasks, err := memory.ListTasks(s.Name)
	if err != nil {
		return err
	}

	task := memory.ResolveDisplayID(tasks, displayID)
	if task == nil {
		return fmt.Errorf("task #%d not found", displayID)
	}

	if err := memory.SetTaskStatus(task.ID, status); err != nil {
		return fmt.Errorf("updating task: %w", err)
	}

	fmt.Printf("Task #%d → %s\n", displayID, status)
	return nil
}

func taskRemove(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: bay task rm <id>")
		return nil
	}

	displayID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	s, err := activeSession()
	if err != nil {
		return err
	}

	tasks, err := memory.ListTasks(s.Name)
	if err != nil {
		return err
	}

	task := memory.ResolveDisplayID(tasks, displayID)
	if task == nil {
		return fmt.Errorf("task #%d not found", displayID)
	}

	if err := memory.DeleteTask(task.ID); err != nil {
		return fmt.Errorf("deleting task: %w", err)
	}

	fmt.Printf("Removed task #%d: %s\n", displayID, task.Title)
	return nil
}

func taskAssign(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: bay task assign <id>")
		return nil
	}

	displayID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" {
		return fmt.Errorf("not in a tmux pane (TMUX_PANE not set)")
	}

	s, err := activeSession()
	if err != nil {
		return err
	}

	tasks, err := memory.ListTasks(s.Name)
	if err != nil {
		return err
	}

	task := memory.ResolveDisplayID(tasks, displayID)
	if task == nil {
		return fmt.Errorf("task #%d not found", displayID)
	}

	// Find the pane in session YAML and set TaskID
	found := false
	for i, p := range s.Panes {
		if p.PaneID == paneID {
			s.Panes[i].TaskID = int(task.ID)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("pane %s not found in session", paneID)
	}

	if err := session.Save(s); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	fmt.Printf("Pane %s assigned to task #%d: %s\n", paneID, displayID, task.Title)
	return nil
}

func taskClear() error {
	s, err := activeSession()
	if err != nil {
		return err
	}

	if err := memory.ClearTasks(s.Name); err != nil {
		return fmt.Errorf("clearing tasks: %w", err)
	}

	fmt.Printf("Cleared all tasks for '%s'\n", s.Name)
	return nil
}
