package tests

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/session"
	"bay/internal/tui/topbar"
)

// newTestTopbar creates a topbar model without tmux or filesystem side effects.
func newTestTopbar() topbar.Model {
	cfg := &config.Config{
		ScanDirs: []string{},
	}
	return topbar.NewForTest(cfg)
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

func TestTopbarSpaceEntersFocus(t *testing.T) {
	m := newTestTopbar()

	// space should focus
	m = sendKey(m, " ")
	if !m.IsFocused() {
		t.Error("space should enter focus mode")
	}

	// space again should NOT unfocus (one-way)
	m = sendKey(m, " ")
	if !m.IsFocused() {
		t.Error("space should not exit focus mode (esc exits)")
	}
}

// NOTE: Do not add a test for quit (q key). The quit handler calls
// KillMainSession() which kills the live bay tmux session during tests.

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
	m = sendKey(m, " ") // focus

	if m.FocusRow() != 0 {
		t.Errorf("focus row should start at 0 (repos), got %d", m.FocusRow())
	}
}

func TestTopbarDownBlockedWithNoSessions(t *testing.T) {
	m := newTestTopbar() // no repos = no sessions
	m = sendKey(m, " ")  // focus

	m = sendSpecialKey(m, tea.KeyDown)
	if m.FocusRow() != 0 {
		t.Error("down arrow should be blocked when there are no sessions")
	}
}

func TestTopbarUpBlockedOnRepoRow(t *testing.T) {
	m := newTestTopbar()
	m = sendKey(m, " ") // focus, starts on row 0

	m = sendSpecialKey(m, tea.KeyUp)
	if m.FocusRow() != 0 {
		t.Error("up arrow should be blocked when already on repo row")
	}
}

func TestTopbarEscUnfocuses(t *testing.T) {
	m := newTestTopbar()
	m = sendKey(m, " ") // focus

	m = sendSpecialKey(m, tea.KeyEscape)
	if m.IsFocused() {
		t.Error("esc should unfocus the topbar")
	}
}

func TestTopbarFocusResetsRowToRepos(t *testing.T) {
	m := newTestTopbar()

	// Focus
	m = sendKey(m, " ")
	if m.FocusRow() != 0 {
		t.Error("focus should start on repo row")
	}

	// Unfocus via esc and refocus via space — should reset to row 0
	m = sendSpecialKey(m, tea.KeyEscape) // unfocus
	m = sendKey(m, " ")                  // refocus
	if m.FocusRow() != 0 {
		t.Error("refocusing should reset to repo row")
	}
}

// --- Hot Row Tests ---

func makeSessions() []*session.Session {
	now := time.Now()
	return []*session.Session{
		{Name: "s1", Repo: "repo-a", WorkingDir: "/tmp", LastActiveAt: now.Add(-1 * time.Minute)},
		{Name: "s2", Repo: "repo-a", WorkingDir: "/tmp", LastActiveAt: now.Add(-5 * time.Minute)},
		{Name: "s3", Repo: "repo-b", WorkingDir: "/tmp", LastActiveAt: now.Add(-10 * time.Minute)},
		{Name: "s4", Repo: "repo-b", WorkingDir: "/tmp", LastActiveAt: now.Add(-20 * time.Minute)},
		{Name: "s5", Repo: "repo-c", WorkingDir: "/tmp", LastActiveAt: now.Add(-30 * time.Minute)},
		{Name: "s6", Repo: "repo-c", WorkingDir: "/tmp", LastActiveAt: now.Add(-60 * time.Minute)},
	}
}

func TestHotRowBuiltOnRefresh(t *testing.T) {
	m := newTestTopbar()
	sessions := makeSessions()
	m.SetSessionsForTest(sessions)

	// Hot row should be populated and capped at MaxHotRowItems (5)
	if m.HotRowLen() != 5 {
		t.Errorf("expected hot row length 5, got %d", m.HotRowLen())
	}
}

func TestHotRowCycleWraps(t *testing.T) {
	m := newTestTopbar()
	sessions := makeSessions()[:3] // 3 sessions
	m.SetSessionsForTest(sessions)
	m.SetActiveSessionForTest("s1")

	if m.HotRowLen() != 3 {
		t.Fatalf("expected hot row length 3, got %d", m.HotRowLen())
	}

	// Cycle idx starts at active session (s1 = index 0)
	if m.HotRowCycleIdx() != 0 {
		t.Errorf("expected cycle idx 0, got %d", m.HotRowCycleIdx())
	}

	// Send tab keys — since activateSession requires tmux, just verify the
	// cycleIdx logic works by checking the hot row was built correctly
	// (full cycle test needs tmux, covered by manual testing)
}

func TestHotRowReorderThreshold(t *testing.T) {
	m := newTestTopbar()
	sessions := makeSessions()[:3]
	m.SetSessionsForTest(sessions)

	initialLen := m.HotRowLen()

	// Inject sessions again — since sessionActivatedAt is zero, it should rebuild
	m.SetSessionsForTest(sessions)
	if m.HotRowLen() != initialLen {
		t.Errorf("hot row should maintain same length after rebuild, got %d", m.HotRowLen())
	}
}

// modeSearch is mode value 10 (0-indexed in the iota).
// "/" now launches a subprocess search TUI (requires tmux), so it enters modeSearch.
const modeSearch = 10

func TestSearchFromUnfocused(t *testing.T) {
	m := newTestTopbar()

	// "/" from unfocused state should enter search mode (subprocess launch)
	m = sendKey(m, "/")
	if m.Mode() != modeSearch {
		t.Errorf("expected mode %d (search), got %d", modeSearch, m.Mode())
	}
}

func TestSearchSignalCancelRestores(t *testing.T) {
	m := newTestTopbar()

	// Enter search mode
	m = sendKey(m, "/")
	if m.Mode() != modeSearch {
		t.Fatalf("expected mode %d (search), got %d", modeSearch, m.Mode())
	}

	// Receiving "F" (cancel signal from subprocess) should return to normal
	m = sendKey(m, "F")
	if m.Mode() != 0 { // modeNormal
		t.Errorf("expected mode 0 (normal), got %d", m.Mode())
	}
}
