package topbar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/config"
	"bay/internal/constants"
	"bay/internal/hooks"
	"bay/internal/logging"
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
	modeCreate
	modeGlobalSearch
	modeCleanup
	modeHelp
)

type clearStatusMsg struct{}
type diffTickMsg struct{}

type diffSummary struct {
	Files      int
	Insertions int
	Deletions  int
	Untracked  int
	Deleted    int
	Clean      bool
	ComputedAt time.Time
}

type diffResultMsg struct {
	SessionName string
	Summary     diffSummary
}

// agentStatusMsg carries polled agent heartbeat data.
type agentStatusMsg struct {
	// session name → last heartbeat time (zero if no heartbeat file)
	heartbeats map[string]time.Time
}

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
	allRepos           []scanner.Repo // unfiltered — all scanned repos
	repos              []scanner.Repo // filtered — only repos with sessions
	sessions           []*session.Session
	focused            bool
	focusRow           int // 0 = repos, 1 = sessions
	activeRepoIdx      int
	selectedSessionIdx int
	plusSelected        bool
	mode               mode
	renameInput        textinput.Model
	noteInput          textinput.Model
	deleteTarget       string
	renameTarget       string
	noteTarget         string
	activeSession      string
	activeWindowIdx    int
	settingsWindowIdx  int
	createWindowIdx    int
	prevWindowIdx      int
	createPreselected  string
	statusMsg          string
	err                error
	width              int
	height             int

	// Hot row state
	hotRow             []*session.Session
	hotRowCycleIdx     int
	sessionActivatedAt time.Time

	// Global search state
	switchInput          textinput.Model
	globalSearchMatches  []*session.Session
	globalSearchSelected int

	// Cleanup state
	cleanupSessions []*session.Session
	cleanupChecked  []bool
	cleanupCursor   int

	// Diff summary cache: session name → diff summary
	diffCache map[string]*diffSummary

	// Agent activity: session name → last heartbeat time
	agentActive map[string]time.Time
}

// New creates a new topbar model.
func New(cfg *config.Config) Model {
	logging.Info("topbar.New: initializing (repos=%d scanDirs=%v)", len(cfg.ScanDirs), cfg.ScanDirs)
	m := newModel(cfg)
	baytmux.InitTopbarPaneID()
	m.refresh()
	logging.Info("topbar.New: loaded %d repos, %d sessions", len(m.repos), len(m.sessions))
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

	si := textinput.New()
	si.Placeholder = "search sessions..."
	si.CharLimit = 100
	si.Width = 40

	return Model{
		cfg:         cfg,
		renameInput: ri,
		noteInput:   ni,
		switchInput: si,
		diffCache:   make(map[string]*diffSummary),
		agentActive: make(map[string]time.Time),
	}
}

// autoActivateMsg triggers auto-activation of the first session on startup.
type autoActivateMsg struct{}

// refresh reloads repos, sessions from disk.
// Repos are filtered to only those with at least one session,
// then sorted by most recent session activity.
func (m *Model) refresh() {
	m.allRepos = scanner.Scan(m.cfg.ScanDirs)
	m.sessions, _ = session.List()

	// Build set of repo names that have sessions
	repoHasSessions := make(map[string]bool)
	for _, s := range m.sessions {
		repoHasSessions[s.Repo] = true
	}

	// Filter repos to only those with sessions
	var filtered []scanner.Repo
	for _, r := range m.allRepos {
		if repoHasSessions[r.Name] {
			filtered = append(filtered, r)
		}
	}

	// Sort filtered repos by most recent session activity (descending)
	sort.Slice(filtered, func(i, j int) bool {
		ti := m.repoLastActive(filtered[i].Name)
		tj := m.repoLastActive(filtered[j].Name)
		return ti.After(tj)
	})

	// Preserve active repo selection across refresh
	prevRepoName := ""
	if m.activeRepoIdx < len(m.repos) && len(m.repos) > 0 {
		prevRepoName = m.repos[m.activeRepoIdx].Name
	}

	// Prepend ~ virtual repo (always visible, pinned first)
	homeDir, _ := os.UserHomeDir()
	tildeRepo := scanner.Repo{Name: "~", Path: homeDir}
	m.repos = append([]scanner.Repo{tildeRepo}, filtered...)

	// Try to keep the same repo selected (offset by 1 for prepended ~)
	m.activeRepoIdx = 0
	m.plusSelected = false
	if prevRepoName != "" {
		for i, r := range m.repos {
			if r.Name == prevRepoName {
				m.activeRepoIdx = i
				break
			}
		}
	}

	if len(m.repos) == 0 {
		m.plusSelected = true
	}

	if m.activeRepoIdx >= len(m.repos) && len(m.repos) > 0 {
		m.activeRepoIdx = 0
	}

	m.maybeUpdateHotRow()
}

// repoLastActive returns the most recent LastActiveAt (or CreatedAt) among sessions for a repo.
func (m *Model) repoLastActive(repoName string) time.Time {
	var latest time.Time
	for _, s := range m.sessions {
		if s.Repo != repoName {
			continue
		}
		t := s.LastActiveAt
		if t.IsZero() {
			t = s.CreatedAt
		}
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

// activeRepoSessions returns sessions belonging to the active repo.
// Stale sessions (missing working directory) are included for visibility.
func (m *Model) activeRepoSessions() []*session.Session {
	if len(m.repos) == 0 || m.plusSelected {
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

// sessionsForRepoIdx returns sessions for the repo at a given index.
func (m *Model) sessionsForRepoIdx(repoIdx int) []*session.Session {
	if repoIdx < 0 || repoIdx >= len(m.repos) {
		return nil
	}
	repoName := m.repos[repoIdx].Name
	var result []*session.Session
	for _, s := range m.sessions {
		if s.Repo == repoName {
			result = append(result, s)
		}
	}
	return result
}

// maxRepoNameWidth returns the length of the longest repo name (for column alignment).
func (m *Model) maxRepoNameWidth() int {
	max := 0
	for _, r := range m.repos {
		if len(r.Name) > max {
			max = len(r.Name)
		}
	}
	return max
}

// gridHeight returns the number of lines for the expanded topbar pane.
func (m *Model) gridHeight() int {
	// header + repo row + session row + 2 border lines
	h := 3 + 2
	// +1 for contextual note if cursor is on a session with a note
	if m.focused && m.focusRow == 1 {
		if m.displayedSessionNote() != "" {
			h++
		}
	}
	return h
}

// sessionsForRepo returns the count of sessions for a given repo name.
func (m *Model) sessionsForRepo(repoName string) int {
	count := 0
	for _, s := range m.sessions {
		if s.Repo == repoName {
			count++
		}
	}
	return count
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
	if len(m.repos) == 0 || m.plusSelected {
		return ""
	}
	return m.repos[m.activeRepoIdx].Name
}

// Init is the Bubbletea init function.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return autoActivateMsg{} },
		tea.Tick(constants.DiffTickInterval, func(time.Time) tea.Msg { return diffTickMsg{} }),
	)
}

// Update handles messages for the topbar.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case autoActivateMsg:
		// Check for stale sessions (older than StaleDays)
		staleCutoff := time.Now().AddDate(0, 0, -constants.StaleDays)
		var staleSessions []*session.Session
		for _, s := range m.sessions {
			t := s.LastActiveAt
			if t.IsZero() {
				t = s.CreatedAt
			}
			if t.Before(staleCutoff) {
				staleSessions = append(staleSessions, s)
			}
		}
		if len(staleSessions) > 0 {
			m.cleanupSessions = staleSessions
			m.cleanupChecked = make([]bool, len(staleSessions))
			for i := range m.cleanupChecked {
				m.cleanupChecked[i] = true
			}
			m.cleanupCursor = 0
			m.mode = modeCleanup
			m.focused = true
			return m, nil
		}
		return m.doAutoActivate()

	case diffResultMsg:
		m.diffCache[msg.SessionName] = &msg.Summary
		return m, nil

	case agentStatusMsg:
		for name, t := range msg.heartbeats {
			m.agentActive[name] = t
		}
		return m, nil

	case diffTickMsg:
		// Refresh diff for the currently displayed session
		cmds := []tea.Cmd{tea.Tick(constants.DiffTickInterval, func(time.Time) tea.Msg { return diffTickMsg{} })}
		if sessionName := m.selectedSessionName(); sessionName != "" {
			cached := m.diffCache[sessionName]
			if cached == nil || time.Since(cached.ComputedAt) > constants.DiffCacheTTL {
				for _, s := range m.sessions {
					if s.Name == sessionName {
						workDir := s.WorkingDir
						cmds = append(cmds, fetchDiffCmd(sessionName, workDir))
						break
					}
				}
			}
		}
		// Poll agent heartbeat files
		cmds = append(cmds, pollAgentStatusCmd())
		return m, tea.Batch(cmds...)

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
			m.plusSelected = false
			m.focusRow = 1
			m.selectedSessionIdx = 0
			for i, r := range m.repos {
				for j, s := range m.sessionsForRepoIdx(i) {
					if s.Name == m.activeSession {
						m.activeRepoIdx = i
						m.selectedSessionIdx = j
						break
					}
				}
				_ = r
			}
			if len(m.activeRepoSessions()) == 0 {
				m.focusRow = 0
			}
			return m, resizeTopbarCmd(m.gridHeight())
		}
		return m, nil

	case tea.BlurMsg:
		m.focused = false
		m.statusMsg = ""
		return m, resizeTopbarCmd(constants.TopbarCollapsedHeight)

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
		if m.mode == modeCreate {
			return m.updateCreate(msg)
		}
		if m.mode == modeGlobalSearch {
			return m.updateGlobalSearch(msg)
		}
		if m.mode == modeCleanup {
			return m.updateCleanup(msg)
		}
		if m.mode == modeHelp {
			m.mode = modeNormal
			return m, nil
		}

		key := msg.String()

		// Clear any lingering status on new keypress
		m.statusMsg = ""

		// Space enters focus — sent by `+Space prefix binding.
		// One-way: space only enters focus mode; esc exits it.
		if key == " " {
			if m.focused {
				return m, nil // already focused — no-op
			}
			m.focused = true
			m.statusMsg = ""
			m.plusSelected = false
			// Default to session row with active session selected
			m.focusRow = 1
			m.selectedSessionIdx = 0
			for i, r := range m.repos {
				for j, s := range m.sessionsForRepoIdx(i) {
					if s.Name == m.activeSession {
						m.activeRepoIdx = i
						m.selectedSessionIdx = j
						break
					}
				}
				_ = r
			}
			// Fall back to repo row if no sessions
			if len(m.activeRepoSessions()) == 0 {
				m.focusRow = 0
			}
			return m, resizeTopbarCmd(m.gridHeight())
		}

		// These work without focus mode (sent via `+Tab / `+0-9 / `+/ prefix bindings).
		// In focus mode, these keys are not used — navigation uses arrow keys instead.
		if !m.focused {
			switch {
			case key == "tab":
				return m.cycleHotRow()
			case key == "/":
				return m.startGlobalSearch()
			case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
				return m.jumpToSession(int(key[0] - '1'))
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
			logging.Info("user quit — killing main session")
			baytmux.KillMainSession()
			return m, tea.Quit
		case "esc":
			m.focused = false
			m.statusMsg = ""
			return m, tea.Batch(unfocusCmd, resizeTopbarCmd(constants.TopbarCollapsedHeight))
		case "down":
			if m.focusRow == 0 && !m.plusSelected {
				if len(m.activeRepoSessions()) > 0 {
					m.focusRow = 1
					m.selectedSessionIdx = 0
				}
			}
			return m, resizeTopbarCmd(m.gridHeight())
		case "up":
			if m.focusRow == 1 {
				m.focusRow = 0
			}
			return m, resizeTopbarCmd(m.gridHeight())
		case "left", "h":
			if m.focusRow == 0 {
				return m.cycleRepoFocused(-1)
			}
			return m.cycleSelectedSession(-1)
		case "right", "l":
			if m.focusRow == 0 {
				return m.cycleRepoFocused(1)
			}
			return m.cycleSelectedSession(1)
		case "enter":
			if m.plusSelected {
				return m.startCreate("")
			}
			if m.focusRow == 0 {
				// Enter on repo row: move to session row
				if len(m.activeRepoSessions()) > 0 {
					m.focusRow = 1
					m.selectedSessionIdx = 0
				}
				return m, nil
			}
			return m.activateSelectedSession()
		case "n":
			if m.plusSelected {
				return m.startCreate("")
			}
			repoName := m.activeRepoName()
			if repoName == "" {
				return m.startCreate("")
			}
			if len(m.activeRepoSessions()) >= constants.MaxSessionsPerRepo {
				m.statusMsg = fmt.Sprintf("Max %d sessions per repo", constants.MaxSessionsPerRepo)
				return m, clearStatusAfter(constants.StatusClearDuration)
			}
			return m.startCreate(repoName)
		case "m":
			if m.focusRow != 1 || len(m.activeRepoSessions()) == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(constants.StatusClearDuration)
			}
			sessionName := m.activeSession
			if sessionName == "" {
				m.statusMsg = "No active session"
				return m, clearStatusAfter(constants.StatusClearDuration)
			}
			return m, func() tea.Msg { return SwitchToMemoryMsg{SessionName: sessionName} }
		case "d":
			if m.focusRow != 1 || len(m.activeRepoSessions()) == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(constants.StatusClearDuration)
			}
			return m.startDelete()
		case "R":
			if m.focusRow != 1 || len(m.activeRepoSessions()) == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(constants.StatusClearDuration)
			}
			return m.startRename()
		case "N":
			if m.focusRow != 1 || len(m.activeRepoSessions()) == 0 {
				m.statusMsg = "Select a session first"
				return m, clearStatusAfter(constants.StatusClearDuration)
			}
			return m.startEditNote()
		case "S":
			return m.startSettings()
		case "/":
			return m.startGlobalSearch()
		case "?":
			m.mode = modeHelp
			return m, nil
		}
	}

	return m, nil
}

func unfocusCmd() tea.Msg {
	baytmux.FocusBelowTopbar()
	return nil
}

// resizeTopbarCmd returns a command that resizes the topbar pane to the given height.
func resizeTopbarCmd(height int) tea.Cmd {
	return func() tea.Msg {
		baytmux.ResizeTopbarPane(height)
		return nil
	}
}

// safeKillWindow breaks the topbar out of a window, then kills it.
// Returns the topbar's new window index, or -1 if the break failed (window not killed).
func safeKillWindow(windowIndex int, context string) int {
	topbarWindow := baytmux.BreakTopbarToOwnWindow()
	if topbarWindow >= 0 {
		baytmux.KillWindow(windowIndex)
	} else {
		logging.Warn("BreakTopbarToOwnWindow failed during %s — skipping KillWindow to protect topbar", context)
	}
	return topbarWindow
}

// returnTopbarToPrev moves the topbar back to the previous session window, if it still exists.
func (m *Model) returnTopbarToPrev() {
	if m.prevWindowIdx > 0 && baytmux.WindowExists(m.prevWindowIdx) {
		baytmux.MoveTopbarToWindow(m.prevWindowIdx)
		baytmux.SwitchToWindow(m.prevWindowIdx)
	}
}

// toTmuxPanes converts session panes to the tmux package's SessionPane type.
func toTmuxPanes(panes []session.Pane) []baytmux.SessionPane {
	result := make([]baytmux.SessionPane, len(panes))
	for i, p := range panes {
		result[i] = baytmux.SessionPane{
			Type:           p.Type,
			Cwd:            p.Cwd,
			Command:        p.Command,
			Title:          p.Title,
			AgentSessionID: p.AgentSessionID,
		}
	}
	return result
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
	if idx >= len(m.hotRow) {
		return m, nil
	}
	return m.activateSession(m.hotRow[idx])
}

func (m Model) cycleSelectedSession(dir int) (tea.Model, tea.Cmd) {
	sessions := m.activeRepoSessions()
	if len(sessions) == 0 {
		return m, nil
	}
	m.selectedSessionIdx = (m.selectedSessionIdx + dir + len(sessions)) % len(sessions)
	return m, resizeTopbarCmd(m.gridHeight())
}

// cycleRepoFocused navigates between repos on the repo row.
func (m Model) cycleRepoFocused(dir int) (tea.Model, tea.Cmd) {
	if m.plusSelected {
		if dir < 0 && len(m.repos) > 0 {
			m.plusSelected = false
			m.activeRepoIdx = len(m.repos) - 1
		} else if dir > 0 && len(m.repos) > 0 {
			m.plusSelected = false
			m.activeRepoIdx = 0
		}
	} else {
		next := m.activeRepoIdx + dir
		if next < 0 {
			m.plusSelected = true
		} else if next >= len(m.repos) {
			m.plusSelected = true
		} else {
			m.activeRepoIdx = next
		}
	}
	m.selectedSessionIdx = 0
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

func (m Model) activateSession(s *session.Session) (tea.Model, tea.Cmd) {
	logging.Info("activating session %q (repo=%s, window=%d)", s.Name, s.Repo, s.TmuxWindow)
	if isSessionStale(s) {
		logging.Warn("session %q has missing directory: %s", s.Name, s.WorkingDir)
		m.statusMsg = "Session directory missing — delete with d"
		return m, clearStatusAfter(constants.StatusClearLong)
	}
	if err := m.switchToSession(s); err != nil {
		logging.Error("switchToSession %q: %v", s.Name, err)
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(constants.StatusClearLong)
	}
	m.focused = false
	m.focusRow = 0
	m.statusMsg = ""

	// Update LastActiveAt timestamp
	s.LastActiveAt = time.Now()
	session.Save(s)

	if err := session.SaveActiveSession(s.Name); err != nil {
		logging.Error("saving active session marker: %v", err)
	}
	m.maybeUpdateHotRow()
	m.sessionActivatedAt = time.Now()
	m.refresh()
	windowIdx := m.activeWindowIdx
	return m, tea.Batch(
		resizeTopbarCmd(constants.TopbarCollapsedHeight),
		func() tea.Msg {
			baytmux.FocusDevPane(windowIdx)
			return tea.ClearScreen()
		},
	)
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
	logging.Info("switchToSession %q: saved window=%d, exists=%v", s.Name, windowIdx, windowIdx != 0 && baytmux.WindowExists(windowIdx))

	coldBoot := false
	if windowIdx == 0 || !baytmux.WindowExists(windowIdx) {
		coldBoot = true
		idx, err := baytmux.CreateSessionWindow(s.WorkingDir)
		if err != nil {
			return fmt.Errorf("creating window: %w", err)
		}
		windowIdx = idx
		s.TmuxWindow = idx
		logging.Info("switchToSession %q: created new window %d", s.Name, windowIdx)
		session.Save(s)
	}

	// Move topbar into the window BEFORE recreating agent panes.
	// The topbar anchors the window — if an agent pane crashes immediately,
	// the window (and session) stays alive.
	logging.Info("switchToSession %q: moving topbar to window %d", s.Name, windowIdx)
	if err := baytmux.MoveTopbarToWindow(windowIdx); err != nil {
		return fmt.Errorf("moving topbar: %w", err)
	}
	logging.Info("switchToSession %q: topbar moved, switching to window %d", s.Name, windowIdx)

	// Recreate panes after the topbar is safely in the window.
	if coldBoot && len(s.Panes) > 0 {
		tmuxPanes := toTmuxPanes(s.Panes)
		baytmux.RecreateSessionPanes(windowIdx, tmuxPanes)
		hooks.SyncPaneLayout(s.Name, windowIdx)
		logging.Info("switchToSession %q: recreated %d panes", s.Name, len(tmuxPanes))
	}

	if err := baytmux.SwitchToWindow(windowIdx); err != nil {
		return fmt.Errorf("switching window: %w", err)
	}

	// Verify the switch landed on the expected window.
	if actual, err := baytmux.CurrentWindowIndex(); err == nil && actual != windowIdx {
		logging.Warn("switchToSession %q: expected window %d but got %d, retrying", s.Name, windowIdx, actual)
		baytmux.SwitchToWindow(windowIdx)
	}
	logging.Info("switchToSession %q: switch complete", s.Name)

	baytmux.FocusTopbarPane()

	m.activeSession = s.Name
	m.activeWindowIdx = windowIdx

	// Activate new session (update working state, log event)
	hooks.OnSessionActivate(s.Name, s.Repo, s.WorkingDir)

	// Clean up orphan tmux windows that don't belong to any session.
	// Run async so it doesn't interfere with the just-activated window.
	go hooks.CleanOrphanWindows()

	return nil
}

// warmBootSession creates the tmux window and panes for a session without
// moving the topbar or changing focus. This allows background sessions to have
// live tmux windows ready for instant switching.
func warmBootSession(s *session.Session) {
	// Skip stale sessions (missing directory)
	if isSessionStale(s) {
		return
	}
	// Skip sessions whose window already exists
	if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
		return
	}

	idx, err := baytmux.CreateSessionWindow(s.WorkingDir)
	if err != nil {
		logging.Warn("warmBootSession %q: create window: %v", s.Name, err)
		return
	}
	s.TmuxWindow = idx
	logging.Info("warmBootSession %q: created window %d", s.Name, idx)

	// Recreate panes if any were saved
	if len(s.Panes) > 0 {
		baytmux.RecreateSessionPanes(idx, toTmuxPanes(s.Panes))
		hooks.SyncPaneLayout(s.Name, idx)
	}

	session.Save(s)
}

func (m Model) startDelete() (tea.Model, tea.Cmd) {
	target := m.selectedSessionName()
	if target == "" {
		m.statusMsg = "No session to delete"
		return m, clearStatusAfter(constants.StatusClearDuration)
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
				topbarWindow = safeKillWindow(s.TmuxWindow, fmt.Sprintf("delete of %q", m.deleteTarget))
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
		return m, clearStatusAfter(constants.StatusClearDuration)
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
		return m, clearStatusAfter(constants.StatusClearDuration)
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
		return m, clearStatusAfter(constants.StatusClearDuration)
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
		return m, clearStatusAfter(constants.StatusClearLong)
	}

	if err := baytmux.MoveTopbarToWindow(newWindowIdx); err != nil {
		baytmux.KillWindow(newWindowIdx)
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(constants.StatusClearLong)
	}

	if err := baytmux.SwitchToWindow(newWindowIdx); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(constants.StatusClearLong)
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
	safeKillWindow(m.settingsWindowIdx, "settings close")

	// Move topbar back to original session window
	m.returnTopbarToPrev()

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

// startCreate launches the create flow in a dev pane (mirrors startSettings pattern).
func (m Model) startCreate(preselectedRepo string) (tea.Model, tea.Cmd) {
	m.prevWindowIdx = m.activeWindowIdx
	m.createPreselected = preselectedRepo

	newWindowIdx, err := baytmux.CreateSessionWindow(config.BayDir())
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(constants.StatusClearLong)
	}

	if err := baytmux.MoveTopbarToWindow(newWindowIdx); err != nil {
		baytmux.KillWindow(newWindowIdx)
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(constants.StatusClearLong)
	}

	if err := baytmux.SwitchToWindow(newWindowIdx); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, clearStatusAfter(constants.StatusClearLong)
	}

	// Build the shell command to run in the dev pane
	bayBin, err := os.Executable()
	if err != nil {
		bayBin = "bay"
	}
	topbarTarget := baytmux.TopbarPaneTarget()

	repoFlag := ""
	if preselectedRepo != "" {
		repoFlag = " '--repo=" + preselectedRepo + "'"
	}

	shellCmd := fmt.Sprintf("%s internal create%s; if [ $? -eq 0 ]; then tmux send-keys -t %s c; else tmux send-keys -t %s C; fi; tmux select-pane -t %s; exit",
		bayBin, repoFlag, topbarTarget, topbarTarget, topbarTarget)
	baytmux.SendToDevPane(newWindowIdx, shellCmd)

	m.mode = modeCreate
	m.createWindowIdx = newWindowIdx
	m.focused = false

	windowIdx := newWindowIdx
	return m, func() tea.Msg {
		baytmux.FocusDevPane(windowIdx)
		return tea.ClearScreen()
	}
}

func (m Model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "c":
		// Success — session was created
		m.mode = modeNormal

		safeKillWindow(m.createWindowIdx, "create")

		// Move topbar back
		m.returnTopbarToPrev()

		// Read created session name and activate it
		createdName := session.LoadCreatedSession()
		_ = session.ClearCreatedSession()
		m.refresh()

		if createdName != "" {
			if s, err := session.Load(createdName); err == nil {
				// Switch to the new session's repo tab
				for i, r := range m.repos {
					if r.Name == s.Repo {
						m.activeRepoIdx = i
						m.plusSelected = false
						break
					}
				}
				m2, cmd := m.activateSession(s)
				if tm, ok := m2.(Model); ok {
					tm.statusMsg = fmt.Sprintf("Created '%s'", createdName)
					return tm, tea.Batch(cmd, clearStatusAfter(2*time.Second))
				}
				return m2, cmd
			}
		}

		m.statusMsg = "Session created"
		m.focused = true
		windowIdx := m.prevWindowIdx
		return m, tea.Batch(
			func() tea.Msg {
				baytmux.FocusDevPane(windowIdx)
				return tea.ClearScreen()
			},
			clearStatusAfter(2*time.Second),
		)

	case "C":
		// Cancel — user escaped the wizard
		m.mode = modeNormal

		safeKillWindow(m.createWindowIdx, "create cancel")

		// Move topbar back
		m.returnTopbarToPrev()

		_ = session.ClearCreatedSession()
		m.refresh()
		m.focused = true

		windowIdx := m.prevWindowIdx
		return m, func() tea.Msg {
			baytmux.FocusDevPane(windowIdx)
			return tea.ClearScreen()
		}
	}

	return m, nil
}

// doAutoActivate performs the standard auto-activation logic (extracted for reuse).
// First warm-boots ALL sessions so they have live tmux windows for instant switching,
// then activates the last-active session normally.
func (m Model) doAutoActivate() (tea.Model, tea.Cmd) {
	// Detect fresh start: only the topbar window exists.
	// Clear stale TmuxWindow values to prevent index collisions —
	// indices from a previous bay run can match windows created
	// by warm boot for different sessions.
	if windows := baytmux.ListWindowIndices(); len(windows) <= 1 {
		for _, s := range m.sessions {
			if s.TmuxWindow != 0 {
				s.TmuxWindow = 0
				session.Save(s)
			}
		}
		m.refresh()
	}

	// Warm boot all sessions so background ones have live tmux windows.
	lastActive := session.LoadActiveSession()
	for _, s := range m.sessions {
		// Skip the session we're about to activate — it will get its window via activateSession.
		if s.Name == lastActive {
			continue
		}
		warmBootSession(s)
	}
	// Reload session data so TmuxWindow values are current in memory.
	m.refresh()

	// Restore last active session if available
	if lastActive != "" {
		if s, err := session.Load(lastActive); err == nil {
			for i, r := range m.repos {
				if r.Name == s.Repo {
					m.activeRepoIdx = i
					m.plusSelected = false
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
	// No sessions at all — show ＋ focused
	if len(m.sessions) == 0 {
		m.focused = true
		m.focusRow = 0
		m.plusSelected = true
	}
	return m, nil
}

// --- Hot Row ---

// buildHotRow sorts all non-stale sessions by LastActiveAt descending,
// caps at MaxHotRowItems, and positions hotRowCycleIdx on the active session.
func (m *Model) buildHotRow() {
	var candidates []*session.Session
	for _, s := range m.sessions {
		if !isSessionStale(s) {
			candidates = append(candidates, s)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		ti := candidates[i].LastActiveAt
		if ti.IsZero() {
			ti = candidates[i].CreatedAt
		}
		tj := candidates[j].LastActiveAt
		if tj.IsZero() {
			tj = candidates[j].CreatedAt
		}
		return ti.After(tj)
	})
	if len(candidates) > constants.MaxHotRowItems {
		candidates = candidates[:constants.MaxHotRowItems]
	}
	m.hotRow = candidates
	m.hotRowCycleIdx = 0
	for i, s := range m.hotRow {
		if s.Name == m.activeSession {
			m.hotRowCycleIdx = i
			break
		}
	}
	// Initialize sessionActivatedAt so the reorder threshold is active
	// immediately after the first build (prevents the first Tab press from
	// reshuffling the hot row).
	if m.sessionActivatedAt.IsZero() {
		m.sessionActivatedAt = time.Now()
	}
}

// maybeUpdateHotRow rebuilds the hot row only if enough time has passed
// since the last session activation (to prevent rapid Tab cycling from reshuffling).
func (m *Model) maybeUpdateHotRow() {
	if time.Since(m.sessionActivatedAt) >= constants.HotRowReorderThreshold {
		m.buildHotRow()
	}
}

// cycleHotRow advances to the next item in the hot row and activates it.
func (m Model) cycleHotRow() (tea.Model, tea.Cmd) {
	if len(m.hotRow) == 0 {
		return m.cycleSession()
	}
	m.hotRowCycleIdx = (m.hotRowCycleIdx + 1) % len(m.hotRow)
	s := m.hotRow[m.hotRowCycleIdx]
	// Switch repo tab to match
	for i, r := range m.repos {
		if r.Name == s.Repo {
			m.activeRepoIdx = i
			m.plusSelected = false
			break
		}
	}
	return m.activateSession(s)
}

// --- Global Search ---

func (m Model) startGlobalSearch() (tea.Model, tea.Cmd) {
	m.mode = modeGlobalSearch
	m.switchInput.SetValue("")
	m.switchInput.Focus()
	m.filterGlobalSearchMatches("")
	return m, textinput.Blink
}

func (m *Model) filterGlobalSearchMatches(query string) {
	query = strings.ToLower(query)
	// Start with MRU-sorted sessions
	var candidates []*session.Session
	for _, s := range m.sessions {
		candidates = append(candidates, s)
	}
	sort.Slice(candidates, func(i, j int) bool {
		ti := candidates[i].LastActiveAt
		if ti.IsZero() {
			ti = candidates[i].CreatedAt
		}
		tj := candidates[j].LastActiveAt
		if tj.IsZero() {
			tj = candidates[j].CreatedAt
		}
		return ti.After(tj)
	})

	var matches []*session.Session
	for _, s := range candidates {
		label := strings.ToLower(s.Repo + "/" + s.Name)
		if query == "" || strings.Contains(label, query) {
			matches = append(matches, s)
		}
	}
	m.globalSearchMatches = matches
	m.globalSearchSelected = 0
}

func (m Model) updateGlobalSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		if !m.focused {
			return m, unfocusCmd
		}
		return m, nil
	case "enter":
		if m.globalSearchSelected < len(m.globalSearchMatches) {
			s := m.globalSearchMatches[m.globalSearchSelected]
			for i, r := range m.repos {
				if r.Name == s.Repo {
					m.activeRepoIdx = i
					m.plusSelected = false
					break
				}
			}
			m.mode = modeNormal
			return m.activateSession(s)
		}
		return m, nil
	case "tab", "down":
		if len(m.globalSearchMatches) > 0 {
			m.globalSearchSelected = (m.globalSearchSelected + 1) % len(m.globalSearchMatches)
		}
		return m, nil
	case "shift+tab", "up":
		if len(m.globalSearchMatches) > 0 {
			m.globalSearchSelected = (m.globalSearchSelected - 1 + len(m.globalSearchMatches)) % len(m.globalSearchMatches)
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.switchInput, cmd = m.switchInput.Update(msg)
		m.filterGlobalSearchMatches(m.switchInput.Value())
		return m, cmd
	}
}

// --- Cleanup ---

func (m Model) updateCleanup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.cleanupSessions = nil
		return m.doAutoActivate()
	case " ":
		if m.cleanupCursor < len(m.cleanupChecked) {
			m.cleanupChecked[m.cleanupCursor] = !m.cleanupChecked[m.cleanupCursor]
		}
		return m, nil
	case "a":
		allChecked := true
		for _, c := range m.cleanupChecked {
			if !c {
				allChecked = false
				break
			}
		}
		for i := range m.cleanupChecked {
			m.cleanupChecked[i] = !allChecked
		}
		return m, nil
	case "up", "k":
		if m.cleanupCursor > 0 {
			m.cleanupCursor--
		}
		return m, nil
	case "down", "j":
		if m.cleanupCursor < len(m.cleanupSessions)-1 {
			m.cleanupCursor++
		}
		return m, nil
	case "enter":
		deletedCount := 0
		for i, s := range m.cleanupSessions {
			if !m.cleanupChecked[i] {
				continue
			}
			loaded, err := session.Load(s.Name)
			if err == nil {
				if loaded.TmuxWindow != 0 && baytmux.WindowExists(loaded.TmuxWindow) {
					safeKillWindow(loaded.TmuxWindow, fmt.Sprintf("cleanup of %q", s.Name))
				}
				if loaded.IsWorktree && loaded.WorktreeBranch != "" {
					worktree.Remove(loaded.RepoPath, loaded.Repo, loaded.WorktreeBranch)
				}
			}
			hooks.OnSessionDelete(s.Name)
			session.Delete(s.Name)
			if m.activeSession == s.Name {
				m.activeSession = ""
				m.activeWindowIdx = 0
			}
			deletedCount++
		}
		m.cleanupSessions = nil
		m.mode = modeNormal
		m.refresh()
		if deletedCount > 0 {
			m.statusMsg = fmt.Sprintf("Cleaned up %d stale session(s)", deletedCount)
		}
		m2, cmd := m.doAutoActivate()
		if deletedCount > 0 {
			if tm, ok := m2.(Model); ok {
				tm.statusMsg = fmt.Sprintf("Cleaned up %d stale session(s)", deletedCount)
				return tm, tea.Batch(cmd, clearStatusAfter(3*time.Second))
			}
		}
		return m2, cmd
	}
	return m, nil
}

// fetchDiffCmd runs git diff --shortstat and git status --porcelain asynchronously.
func fetchDiffCmd(sessionName, workDir string) tea.Cmd {
	return func() tea.Msg {
		// Unstaged changes
		out1, _ := exec.Command("git", "-C", workDir, "diff", "--shortstat").Output()
		f1, i1, d1 := parseShortstat(string(out1))

		// Staged changes
		out2, _ := exec.Command("git", "-C", workDir, "diff", "--cached", "--shortstat").Output()
		f2, i2, d2 := parseShortstat(string(out2))

		files := f1 + f2
		ins := i1 + i2
		del := d1 + d2

		// Count untracked and deleted files from git status
		var untracked, deleted int
		out3, _ := exec.Command("git", "-C", workDir, "status", "--porcelain").Output()
		for _, line := range strings.Split(string(out3), "\n") {
			if len(line) < 2 {
				continue
			}
			prefix := line[:2]
			switch {
			case prefix == "??":
				untracked++
			case prefix[0] == 'D' || prefix[1] == 'D':
				deleted++
			}
		}

		return diffResultMsg{
			SessionName: sessionName,
			Summary: diffSummary{
				Files:      files,
				Insertions: ins,
				Deletions:  del,
				Untracked:  untracked,
				Deleted:    deleted,
				Clean:      files == 0 && ins == 0 && del == 0 && untracked == 0 && deleted == 0,
				ComputedAt: time.Now(),
			},
		}
	}
}

var shortstatRe = regexp.MustCompile(`(\d+) file`)
var insertionsRe = regexp.MustCompile(`(\d+) insertion`)
var deletionsRe = regexp.MustCompile(`(\d+) deletion`)

// parseShortstat parses git diff --shortstat output.
func parseShortstat(output string) (files, ins, del int) {
	if m := shortstatRe.FindStringSubmatch(output); len(m) > 1 {
		files, _ = strconv.Atoi(m[1])
	}
	if m := insertionsRe.FindStringSubmatch(output); len(m) > 1 {
		ins, _ = strconv.Atoi(m[1])
	}
	if m := deletionsRe.FindStringSubmatch(output); len(m) > 1 {
		del, _ = strconv.Atoi(m[1])
	}
	return
}

// pollAgentStatusCmd reads all agent heartbeat files from ~/.bay/agent-status/.
func pollAgentStatusCmd() tea.Cmd {
	return func() tea.Msg {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".bay", "agent-status")
		entries, err := os.ReadDir(dir)
		if err != nil {
			return agentStatusMsg{heartbeats: nil}
		}
		heartbeats := make(map[string]time.Time)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			if err != nil {
				continue
			}
			heartbeats[e.Name()] = time.Unix(ts, 0)
		}
		return agentStatusMsg{heartbeats: heartbeats}
	}
}

// isAgentActive returns true if the session has had a heartbeat within the threshold.
func (m *Model) isAgentActive(sessionName string) bool {
	t, ok := m.agentActive[sessionName]
	if !ok {
		return false
	}
	return time.Since(t) < constants.AgentActiveThreshold
}

// IsFocused returns whether the topbar is in focused mode.
func (m *Model) IsFocused() bool {
	return m.focused
}

// FocusRow returns the currently focused row (0 = repos, 1 = sessions).
func (m *Model) FocusRow() int {
	return m.focusRow
}

// HotRowLen returns the number of items in the hot row.
func (m *Model) HotRowLen() int {
	return len(m.hotRow)
}

// HotRowCycleIdx returns the current cycle index in the hot row.
func (m *Model) HotRowCycleIdx() int {
	return m.hotRowCycleIdx
}

// Mode returns the current mode (for testing).
func (m *Model) Mode() int {
	return int(m.mode)
}

// SetSessionsForTest injects sessions into the model (for testing without filesystem).
func (m *Model) SetSessionsForTest(sessions []*session.Session) {
	m.sessions = sessions
	m.buildHotRow()
}

// SetActiveSessionForTest sets the active session name (for testing).
func (m *Model) SetActiveSessionForTest(name string) {
	m.activeSession = name
}
