package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"bay/internal/config"
	"bay/internal/constants"
	"bay/internal/memory"
	"bay/internal/session"
)

// GenerateUUID generates a v4 UUID using crypto/rand.
func GenerateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating UUID: %w", err)
	}
	// RFC 4122 version 4 UUID: set the 4 high bits of byte 6 to 0100 (version 4),
	// and the 2 high bits of byte 8 to 10 (variant 1). This marks the UUID as
	// randomly generated per the standard, rather than derived from a name or time.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// BuildAgentArgs returns the exec args for launching an agent with session tracking.
// It takes the user's base command (e.g. "claude --dangerously-bypass-permissions")
// and appends the session flag. If resume is true, uses --resume; otherwise --session-id.
func BuildAgentArgs(baseCmd string, uuid string, resume bool) []string {
	parts := strings.Fields(baseCmd)
	if resume {
		parts = append(parts, "--resume", uuid)
	} else {
		parts = append(parts, "--session-id", uuid)
	}
	return parts
}

// Agent launches the configured agent with session tracking.
// Usage: bay agent [--resume <uuid>]
func Agent(args []string) error {
	var resumeID string
	for i := 0; i < len(args); i++ {
		if args[i] == "--resume" && i+1 < len(args) {
			resumeID = args[i+1]
			i++
		}
	}

	// Load the user's configured agent command
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	agentCmd := cfg.Defaults.Agent
	if agentCmd == "" {
		agentCmd = constants.DefaultAgent
	}

	var uuid string
	var resume bool

	if resumeID != "" {
		uuid = resumeID
		resume = true
	} else {
		uuid, err = GenerateUUID()
		if err != nil {
			return err
		}
	}

	// Save the agent session ID to the session YAML.
	// Three strategies: exact PaneID match, first agent pane with empty AgentSessionID, append new.
	paneID := os.Getenv("TMUX_PANE")
	if paneID != "" {
		s, err := session.FindActiveSession()
		if err == nil {
			matched := false
			// Strategy 1: exact PaneID match
			for i, p := range s.Panes {
				if p.PaneID == paneID {
					s.Panes[i].AgentSessionID = uuid
					matched = true
					break
				}
			}
			// Strategy 2: first agent pane with empty AgentSessionID (fresh session / cold boot)
			if !matched {
				for i, p := range s.Panes {
					if p.Type == "agent" && p.AgentSessionID == "" {
						s.Panes[i].AgentSessionID = uuid
						s.Panes[i].PaneID = paneID
						matched = true
						break
					}
				}
			}
			// Strategy 3: append a new pane entry
			if !matched {
				s.Panes = append(s.Panes, session.Pane{
					Type:           "agent",
					PaneID:         paneID,
					AgentSessionID: uuid,
				})
			}
			session.Save(s)
		}
	}

	// Build the full command with session flags appended
	execArgs := BuildAgentArgs(agentCmd, uuid, resume)

	// Find the binary (first element of the command)
	binaryName := execArgs[0]
	binaryPath, err := exec.LookPath(binaryName)
	if err != nil {
		return fmt.Errorf("%s not found in PATH", binaryName)
	}

	// Pass session name to child process so hooks (e.g. agent-heartbeat) know
	// which session this agent belongs to without scanning YAML files.
	env := os.Environ()
	if s, err := session.FindActiveSession(); err == nil {
		env = append(env, "BAY_SESSION="+s.Name)
		// Print session context info before handing off to the agent
		if s.Purpose != "" {
			fmt.Printf("✓ Purpose: %s\n", s.Purpose)
		}
		tasks, _ := memory.ListTasks(s.Name)
		if len(tasks) > 0 {
			pending := 0
			for _, t := range tasks {
				if t.Status != "done" {
					pending++
				}
			}
			fmt.Printf("✓ Checklist: %d items (%d pending)\n", len(tasks), pending)
		}
		if s.Purpose != "" || len(tasks) > 0 {
			fmt.Println("  Context injected via SessionStart hook")
		}
	}

	return syscall.Exec(binaryPath, execArgs, env)
}
