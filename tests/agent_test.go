package tests

import (
	"regexp"
	"testing"

	"bay/cmd"
)

func TestGenerateUUID(t *testing.T) {
	uuid, err := cmd.GenerateUUID()
	if err != nil {
		t.Fatalf("GenerateUUID failed: %v", err)
	}

	// Verify 8-4-4-4-12 hex format
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(uuid) {
		t.Errorf("UUID %q does not match v4 format", uuid)
	}

	// Verify uniqueness
	uuid2, err := cmd.GenerateUUID()
	if err != nil {
		t.Fatalf("second GenerateUUID failed: %v", err)
	}
	if uuid == uuid2 {
		t.Error("two UUIDs should not be identical")
	}
}

func TestAgentCommandFresh(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	args := cmd.BuildAgentArgs("claude", uuid, false)

	expected := []string{"claude", "--session-id", uuid}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestAgentCommandResume(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	args := cmd.BuildAgentArgs("claude", uuid, true)

	expected := []string{"claude", "--resume", uuid}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestAgentCommandWithUserFlags(t *testing.T) {
	uuid := "abc-123"

	// User has custom flags on their agent command
	args := cmd.BuildAgentArgs("claude --dangerously-bypass-permissions", uuid, false)

	expected := []string{"claude", "--dangerously-bypass-permissions", "--session-id", uuid}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestAgentCommandDifferentAgent(t *testing.T) {
	uuid := "abc-123"

	// User configured a different agent
	args := cmd.BuildAgentArgs("codex", uuid, false)

	expected := []string{"codex", "--session-id", uuid}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}
