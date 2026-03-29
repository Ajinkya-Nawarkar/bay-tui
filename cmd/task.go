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
  bay task "purpose text"     Set session purpose
  bay task add "item"         Add checklist item
  bay task ls                 Show purpose + checklist
  bay task done <id>          Mark item done
  bay task rm <id>            Remove item
  bay task clear              Clear all checklist items`)
		return nil
	}

	switch args[0] {
	case "add":
		return taskAdd(args[1:])
	case "ls", "list":
		return taskList()
	case "done":
		return taskSetStatus(args[1:], "done")
	case "undo":
		return taskSetStatus(args[1:], "todo")
	case "rm", "remove":
		return taskRemove(args[1:])
	case "clear":
		return taskClear()
	case "help", "--help", "-h":
		return Task(nil)
	default:
		// Treat as: bay task "purpose text" — set session purpose
		return taskSetPurpose(strings.Join(args, " "))
	}
}

func activeSession() (*session.Session, error) {
	s, err := session.FindActiveSession()
	if err != nil {
		return nil, fmt.Errorf("no active session: %w", err)
	}
	return s, nil
}

func taskSetPurpose(purpose string) error {
	s, err := activeSession()
	if err != nil {
		return err
	}

	s.Purpose = purpose
	if err := session.Save(s); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	fmt.Printf("Purpose set: %s\n", purpose)
	return nil
}

func taskAdd(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `Usage: bay task add "description"`)
		return nil
	}

	title := strings.Join(args, " ")
	s, err := activeSession()
	if err != nil {
		return err
	}

	id, err := memory.CreateTask(s.Name, title)
	if err != nil {
		return fmt.Errorf("creating task: %w", err)
	}

	tasks, err := memory.ListTasks(s.Name)
	if err != nil {
		fmt.Printf("Added: %s\n", title)
		return nil
	}
	displayID := 0
	for i, t := range tasks {
		if t.ID == id {
			displayID = i + 1
			break
		}
	}

	fmt.Printf("Added #%d: %s\n", displayID, title)
	return nil
}

func taskList() error {
	s, err := activeSession()
	if err != nil {
		return err
	}

	if s.Purpose != "" {
		fmt.Printf("Purpose: %s\n\n", s.Purpose)
	}

	tasks, err := memory.ListTasks(s.Name)
	if err != nil {
		return fmt.Errorf("listing tasks: %w", err)
	}

	if len(tasks) == 0 {
		if s.Purpose == "" {
			fmt.Println("No purpose or checklist items set.")
		} else {
			fmt.Println("No checklist items.")
		}
		return nil
	}

	fmt.Println("Checklist:")
	for i, t := range tasks {
		marker := "[ ]"
		if t.Status == "done" {
			marker = "[x]"
		}
		fmt.Printf("  %s %d. %s\n", marker, i+1, t.Title)
	}

	return nil
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
		return fmt.Errorf("item #%d not found", displayID)
	}

	if err := memory.SetTaskStatus(task.ID, status); err != nil {
		return fmt.Errorf("updating task: %w", err)
	}

	fmt.Printf("Item #%d → %s\n", displayID, status)
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
		return fmt.Errorf("item #%d not found", displayID)
	}

	if err := memory.DeleteTask(task.ID); err != nil {
		return fmt.Errorf("deleting task: %w", err)
	}

	fmt.Printf("Removed #%d: %s\n", displayID, task.Title)
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

	fmt.Printf("Cleared checklist for '%s'\n", s.Name)
	return nil
}
