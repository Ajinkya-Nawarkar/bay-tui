package topbar

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/hooks"
	"bay/internal/scanner"
	"bay/internal/session"
	baytmux "bay/internal/tmux"
	"bay/internal/worktree"
)

type mode int

const (
	modeNormal mode = iota
	modeRename
	modeConfirmDelete
	modeEditNote
	modeSettings
)

// SwitchToCreateMsg tells the app to switch to create screen.
type SwitchToCreateMsg struct {
	PreselectedRepo string
}

type clearStatusMsg struct{}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return clearStatusMsg{} })
}

// SwitchToSetupMsg tells the app to switch to setup screen.
type SwitchToSetupMsg struct{}

// SwitchToMemoryMsg tells the app to switch to memory viewer.
type SwitchToMemoryMsg struct {
	SessionName string
}

// Model is the topbar screen state.
type Model struct {
	cfg                *config.Config
	repos              []scanner.Repo
	sessions           []*session.Session
	focused            bool
	focusRow           int // 0 = repos, 1 = sessions
	activeRepoIdx      int
	selectedSessionIdx int
	mode               mode
	renameInput        textinput.Model
	noteInput          textinput.Model
	deleteTarget       string
	renameTarget       string
	noteTarget         string
	activeSession      string
	activeWindowIdx    int
	settingsWindowIdx  int
	prevWindowIdx      int
	statusMsg          string
	err                error
	width              int
	height             int
}

// New creates a new topbar model.
func New(cfg *config.Config) Model {
	m := newModel(cfg)
	baytmux.InitTopbarPaneID()
	m.refresh()
	return m
}

// NewForTest creates a topbar model without tmux or filesystem side effects.
func NewForTest(cfg *config.Config) Model {
	return newModel(cfg)
}

func newModel(cfg *config.Config) Model {
	ri := textinput.New()
	ri.Placeholder = "new-name"
	ri.CharLimit = 100
	ri.Width = 25

	ni := textinput.New()
	ni.Placeholder = "session note"
	ni.CharLimit = 200
	ni.Width = 60

	return Model{
		cfg:         cfg,
		renameInput: ri,
		noteInput:   ni,
	}
}

// autoActivateMsg triggers auto-activation of the first session on startup.
type autoActivateMsg struct{}

// refresh reloads repos, sessions from disk.
func (m *Model) refresh() {
	m.repos = scanner.Scan(m.cfg.ScanDirs)
	m.sessions, _ = session.List()

	if m.activeRepoIdx >= len(m.repos) {
		m.activeRepoIdx = 0
	}
}

// activeRepoSessions returns sessions belonging to the active repo.
// Stale sessions (missing working directory) are included for visibility.
func (m *Model) activeRepoSessions() []*session.Session {
	if len(m.repos) == 0 {
		return nil
	}
	repoName := m.repos[m.activeRepoIdx].Name
	var result []*session.Session
	for _, s := range m.sessions {
		if s.Repo == repoName {
			result = append(result, s)
		}
	}
	return result
}

// isSessionStale returns true if the session's working directory no longer exists.
func isSessionStale(s *session.Session) bool {
	_, err := os.Stat(s.WorkingDir)
	return err != nil
}

// selectedSessionName returns the selected session name when focused on the sessions row,
// otherwise the active session name.
func (m *Model) selectedSessionName() string {
	if m.focused && m.focusRow == 1 {
		sessions := m.activeRepoSessions()
		if m.selectedSessionIdx < len(sessions) {
			return sessions[m.selectedSessionIdx].Name
		}
	}
	return m.activeSession
}

// activeRepoName returns the name of the currently active repo.
func (m *Model) activeRepoName() string {
	if len(m.repos) == 0 {
		return ""
	}
	return m.repos[m.activeRepoIdx].Name
}

// Init is the Bubbletea init function.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return autoActivateMsg{} }
}

// Update handles messages for the topbar.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case autoActivateMsg:
		// Restore last active session if available
		if lastActive := session.LoadActiveSession(); lastActive != "" {
			if s, err := session.Load(lastActive); err == nil {
				// Switch to the correct repo tab
				for i, r := range m.repos {
					if r.Name == s.Repo {
						m.activeRepoIdx = i
						break
					}
				}
				return m.activateSession(s)
			}
		}
		// Fallback: activate first session in current repo
		if sessions := m.activeRepoSessions(); len(sessions) > 0 {
			return m.activateSession(sessions[0])
		}
		return m, nil

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && !m.focused {
			m.focused = true
			m.statusMsg = ""
			sessions := m.activeRepoSessions()
			if len(sessions) > 0 {
				m.focusRow = 1
				for i, s := range sessions {
					if s.Name == m.activeSession {
						m.selectedSessionIdx = i
						break
					}
				}
			} else {
				m.focusRow = 0
			}
		}
		return m, nil

	case tea.BlurMsg:
		if m.focused {
			m.focused = false
			m.focusRow = 0
		}
		return m, nil

	case tea.KeyMsg:
		// Handle modal input modes
		if m.mode == modeRename {
			return m.updateRename(msg)
		}
		if m.mode == modeConfirmDelete {
			return m.updateConfirmDelete(msg)
		}
		if m.mode == modeEditNote {
			return m.updateEditNote(msg)
		}
		if m.mode == modeSettings {
			return m.updateSettings(msg)
		}

		key := msg.String()

		// Clear any lingering status on new keypress
		m.statusMsg = ""

		// Space toggles focus — sent by `+Space prefix binding
		if key == " " {
			m.focused = !m.focused
			m.statusMsg = ""
			if !m.focused {
				m.focusRow = 0
				return m, unfocusCmd
			}
			// Default to sessions row with the active session selected
			sessions := m.activeRepoSessions()
			if len(sessions) > 0 {
				m.focusRow = 1
				for i, s := range sessions {
					if s.Name == m.activeSession {
						m.selectedSessionIdx = i
						break
					}
				}
			} else {
				m.focusRow = 0
			}
			return m, nil
		}

		// These work without focus mode (sent via `+Tab / `+r / `+0-9 prefix bindings).
		// In focus mode, these keys are not used — navigation uses arrow keys instead.
		if !m.focused {
			switch {
			case key == "tab":
				return m.cycleSession()
			case key == "r":
				return m.cycleRepo(1)
			case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
				return m.jumpToSession(int(key[0]-'1'))
			}
		}

		// All other keys require focused mode
		if !m.focused {
			return m, nil
		}

		// Focused-only keybinds
		switch key {
		case "q":
			// Sync ALL sessions with live windows, not just the active one
			allSessions, _ := session.List()
			for _, s := range allSessions {
				if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
					hooks.SyncPaneLayout(s.Name, s.TmuxWindow)
				}
			}
			baytmux.KillMainSession()
			return m, tea.Quit
		case "esc":
			m.focused = false
			m.statusMsg = ""
			return m, unfocusCmd
		case "down":
			if m.focusRow == 0 {
				if sessions := m.activeRepoSessions(); len(sessions) > 0 {
					m.focusRow = 1
				}
			}
			return m, nil
		case "up":
			if m.focusRow == 1 {
				m.focusRow = 0
			}
			return m, nil
		case "left", "h":
			if m.focusRow == 0 {
				return m.cycleRepo(-1)
			}
			return m.cycleSelectedSession(-1)
		case "right", "l":
			if m.focusRow == 0 {
				return m.cycleRepo(1)
			}
			return m.cycleSelectedSession(1)
		case "enter":
			if m.focusRow == 1 {
				return m.activateSelectedSession()
			}
			return m.activateCurrentSession()
		case "n":
			if len(m.activeRepoSessions()) >= 9 {
				m.statusMsg = "Max 9 sessions per repo"
				return m, clearStatusAfter(2 * time.Second)
			}
			repo := m.activeRepoName()
			return m, func() tea.Msg { return SwitchToCreateMsg{PreselectedRepo: repo} }
		case "m":
			if m.focusRow == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(2 * time.Second)
			}
			session := m.activeSession
			if session == "" {
				m.statusMsg = "No active session"
				return m, clearStatusAfter(2 * time.Second)
			}
			return m, func() tea.Msg { return SwitchToMemoryMsg{SessionName: session} }
		case "d":
			if m.focusRow == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(2 * time.Second)
			}
			return m.startDelete()
		case "R":
			if m.focusRow == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(2 * time.Second)
			}
			return m.startRename()
		case "N":
			if m.focusRow == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(2 * time.Second)
			}
			return m.startEditNote()
		case "S":
			return m.startSettings()
		}
	}

	return m, nil
}

func unfocusCmd() tea.Msg {
	baytmux.FocusBelowTopbar()
	return nil
}

func (m Model) cycleSession() (tea.Model, tea.Cmd) {
	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		return m, nil
	}

	currentIdx := -1
	for i, s := range sessions {
		if s.Name == m.activeSession {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(sessions)
	return m.activateSession(sessions[nextIdx])
}

func (m Model) jumpToSession(idx int) (tea.Model, tea.Cmd) {
	sessions := m.activeRepoSessions()
	if idx >= len(sessions) {
		return m, nil
	}
	return m.activateSession(sessions[idx])
}

func (m Model) cycleRepo(dir int) (tea.Model, tea.Cmd) {
	if len(m.repos) == 0 {
		return m, nil
	}
	m.activeRepoIdx = (m.activeRepoIdx + dir + len(m.repos)) % len(m.repos)
	m.selectedSessionIdx = 0
	m.statusMsg = ""
	return m, nil
}

func (m Model) cycleSelectedSession(dir int) (tea.Model, tea.Cmd) {
	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		return m, nil
	}
	m.selectedSessionIdx = (m.selectedSessionIdx + dir + len(sessions)) % len(sessions)
	return m, nil
}

func (m Model) activateSelectedSession() (tea.Model, tea.Cmd) {
	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		return m, nil
	}
	if m.selectedSessionIdx >= len(sessions) {
		m.selectedSessionIdx = 0
	}
	return m.activateSession(sessions[m.selectedSessionIdx])
}

func (m Model) activateCurrentSession() (tea.Model, tea.Cmd) {
	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		return m, nil
	}
	// Activate the first session in the current repo, or the already-active one
	for _, s := range sessions {
		if s.Name == m.activeSession {
			return m.activateSession(s)
		}
	}
	return m.activateSession(sessions[0])
}

func (m Model) activateSession(s *session.Session) (tea.Model, tea.Cmd) {
	if isSessionStale(s) {
		m.statusMsg = "Session directory missing — delete with d"
		return m, clearStatusAfter(3 * time.Second)
	}
	if err := m.switchToSession(s); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(3 * time.Second)
	}
	m.focused = false
	m.focusRow = 0
	m.statusMsg = ""
	session.SaveActiveSession(s.Name)
	m.refresh()
	windowIdx := m.activeWindowIdx
	return m, func() tea.Msg {
		baytmux.FocusDevPane(windowIdx)
		return tea.ClearScreen()
	}
}

// switchToSession handles tmux window management for activating a session.
func (m *Model) switchToSession(s *session.Session) error {
	// Deactivate previous session (capture buffers, log event)
	if m.activeSession != "" && m.activeSession != s.Name {
		prevSession, err := session.Load(m.activeSession)
		if err == nil {
			hooks.OnSessionDeactivate(m.activeSession, prevSession.RepoPath, m.activeWindowIdx)
		}
	}

	windowIdx := s.TmuxWindow

	if windowIdx == 0 || !baytmux.WindowExists(windowIdx) {
		idx, err := baytmux.CreateSessionWindow(s.WorkingDir)
		if err != nil {
			return fmt.Errorf("creating window: %w", err)
		}
		windowIdx = idx
		s.TmuxWindow = idx

		// Recreate panes from saved layout (cold boot recovery)
		if len(s.Panes) > 0 {
			var tmuxPanes []baytmux.SessionPane
			for _, p := range s.Panes {
				tmuxPanes = append(tmuxPanes, baytmux.SessionPane{
					Type:    p.Type,
					Cwd:     p.Cwd,
					Command: p.Command,
					Title:   p.Title,
				})
			}
			baytmux.RecreateSessionPanes(windowIdx, tmuxPanes)
		}

		session.Save(s)
	}

	if err := baytmux.MoveTopbarToWindow(windowIdx); err != nil {
		return fmt.Errorf("moving topbar: %w", err)
	}

	if err := baytmux.SwitchToWindow(windowIdx); err != nil {
		return fmt.Errorf("switching window: %w", err)
	}

	baytmux.FocusTopbarPane()

	m.activeSession = s.Name
	m.activeWindowIdx = windowIdx

	// Activate new session (update working state, log event)
	hooks.OnSessionActivate(s.Name, s.Repo, s.WorkingDir)

	// Clean up orphan tmux windows that don't belong to any session
	hooks.CleanOrphanWindows()

	return nil
}

func (m Model) startDelete() (tea.Model, tea.Cmd) {
	target := m.selectedSessionName()
	if target == "" {
		m.statusMsg = "No session to delete"
		return m, clearStatusAfter(2 * time.Second)
	}
	m.mode = modeConfirmDelete
	m.deleteTarget = target
	return m, nil
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeNormal
		deletedName := m.deleteTarget
		topbarWindow := -1
		s, err := session.Load(m.deleteTarget)
		if err == nil {
			if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
				// Move topbar out before killing to avoid killing it along with the window.
				topbarWindow = baytmux.BreakTopbarToOwnWindow()
				baytmux.KillWindow(s.TmuxWindow)
			}
			if s.IsWorktree && s.WorktreeBranch != "" {
				worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch)
			}
		}
		hooks.OnSessionDelete(m.deleteTarget)
		session.Delete(m.deleteTarget)
		if m.activeSession == m.deleteTarget {
			m.activeSession = ""
			m.activeWindowIdx = 0
		}
		m.deleteTarget = ""
		m.refresh()

		// Auto-activate another session so the screen isn't empty.
		// Try current repo first, then any remaining session. Skip stale sessions.
		nextSessions := m.activeRepoSessions()
		if len(nextSessions) == 0 {
			if all, lerr := session.List(); lerr == nil && len(all) > 0 {
				nextSessions = append(nextSessions, all[0])
			}
		}
		// Find the first non-stale session
		var nextSession *session.Session
		for _, ns := range nextSessions {
			if !isSessionStale(ns) {
				nextSession = ns
				break
			}
		}
		if nextSession != nil {
			m2, activateCmd := m.activateSession(nextSession)
			if tm, ok := m2.(Model); ok {
				tm.statusMsg = fmt.Sprintf("Deleted '%s'", deletedName)
				return tm, tea.Batch(activateCmd, clearStatusAfter(2*time.Second))
			}
			return m2, activateCmd
		}

		// No sessions left — switch to the topbar's standalone window.
		m.statusMsg = fmt.Sprintf("Deleted '%s'", deletedName)
		tw := topbarWindow
		return m, tea.Batch(
			func() tea.Msg {
				if tw >= 0 {
					baytmux.SelectWindow(tw)
				}
				return tea.ClearScreen()
			},
			clearStatusAfter(2*time.Second),
		)
	case "n", "N", "esc":
		m.mode = modeNormal
		m.deleteTarget = ""
	}
	return m, nil
}

func (m Model) startRename() (tea.Model, tea.Cmd) {
	target := m.selectedSessionName()
	if target == "" {
		m.statusMsg = "No session to rename"
		return m, clearStatusAfter(2 * time.Second)
	}
	m.mode = modeRename
	m.renameTarget = target
	m.renameInput.SetValue(target)
	m.renameInput.Focus()
	return m, textinput.Blink
}

func (m Model) updateRename(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		oldName := m.renameTarget
		newName := m.renameInput.Value()
		if newName != "" && newName != oldName {
			if err := session.Rename(oldName, newName); err != nil {
				m.statusMsg = fmt.Sprintf("Rename error: %v", err)
			} else {
				hooks.OnSessionRename(oldName, newName)
				if m.activeSession == oldName {
					m.activeSession = newName
				}
				m.statusMsg = fmt.Sprintf("Renamed to '%s'", newName)
				m.refresh()
			}
		}
		m.mode = modeNormal
		m.renameTarget = ""
		return m, clearStatusAfter(2 * time.Second)
	case "esc":
		m.mode = modeNormal
		m.renameTarget = ""
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

func (m Model) startEditNote() (tea.Model, tea.Cmd) {
	target := m.selectedSessionName()
	if target == "" {
		m.statusMsg = "No session"
		return m, clearStatusAfter(2 * time.Second)
	}
	m.mode = modeEditNote
	m.noteTarget = target
	// Pre-fill with existing note
	s, err := session.Load(target)
	if err == nil {
		m.noteInput.SetValue(s.Note)
	} else {
		m.noteInput.SetValue("")
	}
	m.noteInput.Focus()
	return m, textinput.Blink
}

func (m Model) updateEditNote(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		note := m.noteInput.Value()
		s, err := session.Load(m.noteTarget)
		if err == nil {
			s.Note = note
			session.Save(s)
		}
		m.mode = modeNormal
		m.noteTarget = ""
		m.refresh()
		return m, nil
	case "esc":
		m.mode = modeNormal
		m.noteTarget = ""
		return m, nil
	default:
		var cmd tea.Cmd
		m.noteInput, cmd = m.noteInput.Update(msg)
		return m, cmd
	}
}

// displayedSessionNote returns the note for the session currently shown in the topbar.
// In focus mode on the sessions row, this is the selected session; otherwise the active session.
func (m *Model) displayedSessionNote() string {
	// Don't show note when browsing repos
	if m.focused && m.focusRow == 0 {
		return ""
	}
	target := m.selectedSessionName()
	if target == "" {
		return ""
	}
	for _, s := range m.sessions {
		if s.Name == target {
			return s.Note
		}
	}
	return ""
}

// Refresh reloads data (called from parent after returning from create/setup).
func (m *Model) Refresh() {
	cfg, err := config.Load()
	if err == nil {
		m.cfg = cfg
	}
	m.refresh()
}

// SetStatus sets a status message.
func (m *Model) SetStatus(msg string) {
	m.statusMsg = msg
}

// ActivateSession switches to a session by name.
func (m *Model) ActivateSession(name string) {
	s, err := session.Load(name)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return
	}

	if err := m.switchToSession(s); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return
	}

	m.refresh()
}

func (m Model) startSettings() (tea.Model, tea.Cmd) {
	m.prevWindowIdx = m.activeWindowIdx

	newWindowIdx, err := baytmux.CreateSessionWindow(config.BayDir())
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(3 * time.Second)
	}

	if err := baytmux.MoveTopbarToWindow(newWindowIdx); err != nil {
		baytmux.KillWindow(newWindowIdx)
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(3 * time.Second)
	}

	if err := baytmux.SwitchToWindow(newWindowIdx); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(3 * time.Second)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	topbarTarget := baytmux.TopbarPaneTarget()
	cmd := fmt.Sprintf("%s %s; tmux select-pane -t %s; tmux send-keys -t %s s; exit",
		editor, config.ConfigPath(), topbarTarget, topbarTarget)
	baytmux.SendToDevPane(newWindowIdx, cmd)

	m.mode = modeSettings
	m.settingsWindowIdx = newWindowIdx
	m.focused = false

	windowIdx := newWindowIdx
	return m, func() tea.Msg {
		baytmux.FocusDevPane(windowIdx)
		return tea.ClearScreen()
	}
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "s" {
		return m, nil
	}

	// Trigger from chained shell command — editor has closed
	m.mode = modeNormal

	// Break topbar out before killing the settings window
	baytmux.BreakTopbarToOwnWindow()
	baytmux.KillWindow(m.settingsWindowIdx)

	// Move topbar back to original session window
	if m.prevWindowIdx > 0 && baytmux.WindowExists(m.prevWindowIdx) {
		baytmux.MoveTopbarToWindow(m.prevWindowIdx)
		baytmux.SwitchToWindow(m.prevWindowIdx)
	}

	// Reload config and rebind keys
	if cfg, err := config.Load(); err == nil {
		m.cfg = cfg
		baytmux.BindKeys(cfg.Defaults.Agent)
	}
	m.refresh()
	m.statusMsg = "Settings reloaded"
	m.focused = true

	windowIdx := m.prevWindowIdx
	return m, tea.Batch(
		func() tea.Msg {
			baytmux.FocusDevPane(windowIdx)
			return tea.ClearScreen()
		},
		clearStatusAfter(2*time.Second),
	)
}

// IsFocused returns whether the topbar is in focused mode.
func (m *Model) IsFocused() bool {
	return m.focused
}

// FocusRow returns the currently focused row (0 = repos, 1 = sessions).
func (m *Model) FocusRow() int {
	return m.focusRow
}
