package setup

import (
	"encoding/json"
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
	worktreeChoice   int // 0 = managed, 1 = adjacent
	cfg              *config.Config
	err              error
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

	// Resolve full binary path for the hook command
	hookCmd := "bay context"
	if exe, err := os.Executable(); err == nil {
		hookCmd = exe + " context"
	}

	// Check existing SessionStart hooks
	if existing, ok := hooks["SessionStart"]; ok {
		// Check if bay context is already configured (matches both "bay context" and full path)
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

	// Add bay context hook with full binary path
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

	return Model{
		step:         stepWelcome,
		scanDirInput: ti,
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
				// Save config
				if err := config.Save(m.cfg); err != nil {
					m.err = err
					return m, nil
				}
				// Auto-configure Claude Code SessionStart hook
				ensureClaudeHook()
				m.step = stepDone
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
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
