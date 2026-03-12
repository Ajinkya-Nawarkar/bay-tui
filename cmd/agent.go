package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"bay/internal/config"
	"bay/internal/session"
)

// GenerateUUID generates a v4 UUID using crypto/rand.
func GenerateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating UUID: %w", err)
	}
	// Set version 4 and variant bits
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
		agentCmd = "claude"
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

	// Save the agent session ID to the session YAML
	paneID := os.Getenv("TMUX_PANE")
	if paneID != "" {
		s, err := session.FindActiveSession()
		if err == nil {
			for i, p := range s.Panes {
				if p.PaneID == paneID {
					s.Panes[i].AgentSessionID = uuid
					break
				}
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

	return syscall.Exec(binaryPath, execArgs, os.Environ())
}
