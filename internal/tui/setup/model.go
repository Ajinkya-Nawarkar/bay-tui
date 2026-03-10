package setup

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
)

type step int

const (
	stepWelcome step = iota
	stepScanDir
	stepWorktreeLocation
	stepAgentCmd
	stepDone
)

// DoneMsg is sent when setup is complete.
type DoneMsg struct {
	Config *config.Config
}

// Model is the setup wizard state.
type Model struct {
	step             step
	scanDirInput     textinput.Model
	agentInput       textinput.Model
	worktreeChoice   int // 0 = managed, 1 = adjacent
	cfg              *config.Config
	err              error
}

// installDir returns ~/.local/bin.
func installDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

// installBinary copies the current bay binary to ~/.local/bin/bay and ensures
// ~/.local/bin is in the user's shell PATH.
func installBinary() error {
	dest := filepath.Join(installDir(), "bay")

	// Already installed here — skip copy
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			if resolved == dest {
				return nil
			}
		}
	}

	if err := os.MkdirAll(installDir(), 0755); err != nil {
		return err
	}

	// Copy current binary to install dir
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	src, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// Ensure ~/.local/bin is in PATH via shell rc
	ensurePath()
	return nil
}

// ensurePath adds ~/.local/bin to PATH in the user's shell rc file if not already present.
func ensurePath() {
	home, _ := os.UserHomeDir()
	pathLine := `export PATH="$HOME/.local/bin:$PATH"`

	// Try zshrc first, fall back to bashrc
	rcFile := filepath.Join(home, ".zshrc")
	if _, err := os.Stat(rcFile); err != nil {
		rcFile = filepath.Join(home, ".bashrc")
	}

	data, err := os.ReadFile(rcFile)
	if err != nil {
		return
	}

	// Already has it
	if strings.Contains(string(data), ".local/bin") {
		return
	}

	// Append
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString("\n# bay — added by bay setup\n" + pathLine + "\n")
}

// ensureClaudeHook adds the bay context SessionStart hook to ~/.claude/settings.json.
func ensureClaudeHook() {
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	// Read existing settings (or start fresh)
	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}

	// Check if hooks.SessionStart already has bay context
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	// Always use the stable install path
	hookCmd := filepath.Join(installDir(), "bay") + " context"

	// Check existing SessionStart hooks
	if existing, ok := hooks["SessionStart"]; ok {
		if arr, ok := existing.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if innerHooks, ok := m["hooks"].([]any); ok {
						for _, h := range innerHooks {
							if hm, ok := h.(map[string]any); ok {
								if cmd, _ := hm["command"].(string); strings.HasSuffix(cmd, "bay context") {
									return // Already configured
								}
							}
						}
					}
				}
			}
		}
	}

	// Add bay context hook
	bayHook := map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCmd,
			},
		},
	}

	// Append to existing or create new
	if existing, ok := hooks["SessionStart"].([]any); ok {
		hooks["SessionStart"] = append(existing, bayHook)
	} else {
		hooks["SessionStart"] = []any{bayHook}
	}

	settings["hooks"] = hooks

	// Write back
	os.MkdirAll(filepath.Dir(settingsPath), 0755)
	out, err := json.MarshalIndent(settings, "", "  ")
	if err == nil {
		os.WriteFile(settingsPath, out, 0644)
	}
}

// New creates a new setup wizard model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "/Users/you/workspace"
	ti.CharLimit = 256
	ti.Width = 50

	// Pre-fill with ~/workspace if it exists
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, "workspace")
	if _, err := os.Stat(defaultDir); err == nil {
		ti.SetValue(defaultDir)
	}

	ti.Focus()

	ai := textinput.New()
	ai.Placeholder = "claude, codex, gemini, etc."
	ai.CharLimit = 256
	ai.Width = 50
	ai.SetValue("claude")

	return Model{
		step:         stepWelcome,
		scanDirInput: ti,
		agentInput:   ai,
		cfg:          config.DefaultConfig(),
	}
}

// Init starts the text input blink.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input for the setup wizard.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepWelcome:
			if msg.String() == "enter" {
				m.step = stepScanDir
				return m, textinput.Blink
			}
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

		case stepScanDir:
			switch msg.String() {
			case "enter":
				dir := m.scanDirInput.Value()
				if dir != "" {
					m.cfg.ScanDirs = []string{dir}
				}
				m.step = stepWorktreeLocation
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				var cmd tea.Cmd
				m.scanDirInput, cmd = m.scanDirInput.Update(msg)
				return m, cmd
			}

		case stepWorktreeLocation:
			switch msg.String() {
			case "1":
				m.cfg.Defaults.WorktreeLocation = "managed"
				m.worktreeChoice = 0
			case "2":
				m.cfg.Defaults.WorktreeLocation = "adjacent"
				m.worktreeChoice = 1
			case "enter":
				m.step = stepAgentCmd
				m.agentInput.Focus()
				return m, textinput.Blink
			case "ctrl+c":
				return m, tea.Quit
			}

		case stepAgentCmd:
			switch msg.String() {
			case "enter":
				agent := m.agentInput.Value()
				if agent != "" {
					m.cfg.Defaults.Agent = agent
				}
				// Save config
				if err := config.Save(m.cfg); err != nil {
					m.err = err
					return m, nil
				}
				// Install binary to ~/.local/bin and add to PATH
				installBinary()
				// Auto-configure Claude Code SessionStart hook
				ensureClaudeHook()
				m.step = stepDone
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				var cmd tea.Cmd
				m.agentInput, cmd = m.agentInput.Update(msg)
				return m, cmd
			}

		case stepDone:
			if msg.String() == "enter" || msg.String() == "q" {
				return m, func() tea.Msg {
					return DoneMsg{Config: m.cfg}
				}
			}
		}
	}

	return m, nil
}
