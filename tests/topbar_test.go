package tests

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anawarkar/bay/internal/config"
	"github.com/anawarkar/bay/internal/tui/topbar"
)

// newTestTopbar creates a topbar model with empty config (no scan dirs = no repos).
func newTestTopbar() topbar.Model {
	cfg := &config.Config{
		ScanDirs: []string{}, // no repos — avoids tmux calls
	}
	return topbar.New(cfg)
}

func sendKey(m topbar.Model, key string) topbar.Model {
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return result.(topbar.Model)
}

func sendSpecialKey(m topbar.Model, keyType tea.KeyType) topbar.Model {
	result, _ := m.Update(tea.KeyMsg{Type: keyType})
	return result.(topbar.Model)
}

func TestTopbarStartsUnfocused(t *testing.T) {
	m := newTestTopbar()
	if m.IsFocused() {
		t.Error("topbar should start unfocused")
	}
}

func TestTopbarQTogglesFocus(t *testing.T) {
	m := newTestTopbar()

	// q should focus
	m = sendKey(m, "q")
	if !m.IsFocused() {
		t.Error("q should toggle focus on")
	}

	// q again should unfocus
	m = sendKey(m, "q")
	if m.IsFocused() {
		t.Error("q should toggle focus off")
	}
}

func TestTopbarIgnoresKeysWhenUnfocused(t *testing.T) {
	m := newTestTopbar()

	// Focused-only keys should be ignored when unfocused
	for _, key := range []string{"h", "l", "n", "d", "R"} {
		m2 := sendKey(m, key)
		if m2.IsFocused() {
			t.Errorf("key '%s' should not change focus when unfocused", key)
		}
	}
}

func TestTopbarFocusRowStartsAtRepos(t *testing.T) {
	m := newTestTopbar()
	m = sendKey(m, "q") // focus

	if m.FocusRow() != 0 {
		t.Errorf("focus row should start at 0 (repos), got %d", m.FocusRow())
	}
}

func TestTopbarDownBlockedWithNoSessions(t *testing.T) {
	m := newTestTopbar() // no repos = no sessions
	m = sendKey(m, "q")  // focus

	m = sendSpecialKey(m, tea.KeyDown)
	if m.FocusRow() != 0 {
		t.Error("down arrow should be blocked when there are no sessions")
	}
}

func TestTopbarUpBlockedOnRepoRow(t *testing.T) {
	m := newTestTopbar()
	m = sendKey(m, "q") // focus, starts on row 0

	m = sendSpecialKey(m, tea.KeyUp)
	if m.FocusRow() != 0 {
		t.Error("up arrow should be blocked when already on repo row")
	}
}

func TestTopbarEscUnfocuses(t *testing.T) {
	m := newTestTopbar()
	m = sendKey(m, "q") // focus

	m = sendSpecialKey(m, tea.KeyEscape)
	if m.IsFocused() {
		t.Error("esc should unfocus the topbar")
	}
}

func TestTopbarFocusResetsRowToRepos(t *testing.T) {
	m := newTestTopbar()

	// Focus
	m = sendKey(m, "q")
	if m.FocusRow() != 0 {
		t.Error("focus should start on repo row")
	}

	// Unfocus and refocus — should reset to row 0
	m = sendKey(m, "q") // unfocus
	m = sendKey(m, "q") // refocus
	if m.FocusRow() != 0 {
		t.Error("refocusing should reset to repo row")
	}
}
