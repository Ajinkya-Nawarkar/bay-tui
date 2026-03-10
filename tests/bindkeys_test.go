package tests

import (
	"strings"
	"testing"

	baytmux "bay/internal/tmux"
)

// capturedCmd records a single tmux command invocation.
type capturedCmd struct {
	args []string
}

func captureBindKeys() ([]capturedCmd, error) {
	return captureBindKeysWithAgent("claude")
}

func captureBindKeysWithAgent(agentCmd string) ([]capturedCmd, error) {
	var cmds []capturedCmd
	runner := func(args ...string) (string, error) {
		cmds = append(cmds, capturedCmd{args: args})
		return "", nil
	}
	err := baytmux.BindKeysWithRunner(runner, agentCmd)
	return cmds, err
}

// findCmd finds all commands where args[0] matches the given subcmd.
func findCmds(cmds []capturedCmd, subcmd string) []capturedCmd {
	var result []capturedCmd
	for _, c := range cmds {
		if len(c.args) > 0 && c.args[0] == subcmd {
			result = append(result, c)
		}
	}
	return result
}

// findBindKey finds bind-key commands for a specific key.
func findBindKey(cmds []capturedCmd, key string) *capturedCmd {
	for _, c := range cmds {
		if len(c.args) < 2 || c.args[0] != "bind-key" {
			continue
		}
		// bind-key may have flags like -r before the key
		for i := 1; i < len(c.args); i++ {
			if c.args[i] == "-r" {
				continue
			}
			if c.args[i] == key {
				return &c
			}
			break
		}
	}
	return nil
}

func argsContain(args []string, substr string) bool {
	for _, a := range args {
		if strings.Contains(a, substr) {
			return true
		}
	}
	return false
}

func TestBindKeysNoError(t *testing.T) {
	_, err := captureBindKeys()
	if err != nil {
		t.Fatalf("BindKeysWithRunner returned error: %v", err)
	}
}

func TestBindKeysStatusLeftCMDOverlay(t *testing.T) {
	cmds, _ := captureBindKeys()

	for _, c := range cmds {
		if len(c.args) >= 4 && c.args[0] == "set-option" && c.args[2] == "status-left" {
			val := c.args[3]
			if !strings.Contains(val, "client_prefix") {
				t.Error("status-left should use client_prefix conditional")
			}
			if !strings.Contains(val, "⌘ CMD") {
				t.Error("status-left should contain '⌘ CMD' for prefix active state")
			}
			if !strings.Contains(val, "bay") {
				t.Error("status-left should contain 'bay' for normal state")
			}
			return
		}
	}
	t.Error("no status-left set-option found")
}

func TestBindKeysStatusLeftStyleCMDOverlay(t *testing.T) {
	cmds, _ := captureBindKeys()

	for _, c := range cmds {
		if len(c.args) >= 4 && c.args[0] == "set-option" && c.args[2] == "status-left-style" {
			val := c.args[3]
			if !strings.Contains(val, "client_prefix") {
				t.Error("status-left-style should use client_prefix conditional")
			}
			if !strings.Contains(val, "#7C3AED") {
				t.Error("status-left-style should contain purple (#7C3AED) for prefix active state")
			}
			if !strings.Contains(val, "#06B6D4") {
				t.Error("status-left-style should contain cyan (#06B6D4) for normal state")
			}
			return
		}
	}
	t.Error("no status-left-style set-option found")
}

func TestBindKeysAgentSplit(t *testing.T) {
	cmds, _ := captureBindKeys()
	c := findBindKey(cmds, "a")
	if c == nil {
		t.Fatal("no bind-key for 'a' found")
	}
	if !argsContain(c.args, "claude") {
		t.Error("agent split binding should contain 'claude'")
	}
	if !argsContain(c.args, "split-window") {
		t.Error("agent split binding should use split-window")
	}
}

func TestBindKeysCustomAgent(t *testing.T) {
	cmds, _ := captureBindKeysWithAgent("codex")
	c := findBindKey(cmds, "a")
	if c == nil {
		t.Fatal("no bind-key for 'a' found")
	}
	if !argsContain(c.args, "codex") {
		t.Error("agent split binding should contain configured agent 'codex'")
	}
	if argsContain(c.args, "claude") {
		t.Error("agent split binding should not contain 'claude' when configured with 'codex'")
	}
}

func TestBindKeysQuickAccessUsesDynamicTarget(t *testing.T) {
	cmds, _ := captureBindKeys()

	// `+Space should use .0 not a hardcoded pane target
	c := findBindKey(cmds, "Space")
	if c == nil {
		t.Fatal("no bind-key for 'Space' found")
	}
	if !argsContain(c.args, ".0") {
		t.Error("`+Space binding should target .0 (dynamic pane 0)")
	}
	// Should NOT contain a hardcoded bay:0.0 target
	for _, a := range c.args {
		if strings.Contains(a, "bay:0.0") {
			t.Error("`+Space binding should not use hardcoded bay:0.0")
		}
	}

	// `+Tab should use .0
	tab := findBindKey(cmds, "Tab")
	if tab == nil {
		t.Fatal("no bind-key for 'Tab' found")
	}
	if !argsContain(tab.args, ".0") {
		t.Error("`+Tab binding should target .0")
	}

	// `+r should use .0
	r := findBindKey(cmds, "r")
	if r == nil {
		t.Fatal("no bind-key for 'r' found")
	}
	if !argsContain(r.args, ".0") {
		t.Error("`+r binding should target .0")
	}

	// `+1 through `+9 should use .0
	for i := 1; i <= 9; i++ {
		key := strings.TrimSpace(strings.Repeat(" ", 0) + string(rune('0'+i)))
		c := findBindKey(cmds, key)
		if c == nil {
			t.Errorf("no bind-key for '%s' found", key)
			continue
		}
		if !argsContain(c.args, ".0") {
			t.Errorf("`+%s binding should target .0", key)
		}
	}
}

func TestBindKeysPrefixIsBacktick(t *testing.T) {
	cmds, _ := captureBindKeys()
	for _, c := range cmds {
		if len(c.args) >= 4 && c.args[0] == "set-option" && c.args[2] == "prefix2" {
			if c.args[3] != "`" {
				t.Errorf("expected prefix2 to be backtick, got '%s'", c.args[3])
			}
			return
		}
	}
	t.Error("no prefix2 set-option found")
}

func TestBindKeysNumberOfBindings(t *testing.T) {
	cmds, _ := captureBindKeys()
	binds := findCmds(cmds, "bind-key")
	// At minimum: `, Left, Right, Up, Down, d, D, a, w, s, q, Tab, r, 0-9 = 24+
	if len(binds) < 20 {
		t.Errorf("expected at least 20 bind-key commands, got %d", len(binds))
	}
}
