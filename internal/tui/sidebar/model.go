package sidebar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anawarkar/bay/internal/config"
	"github.com/anawarkar/bay/internal/scanner"
	"github.com/anawarkar/bay/internal/session"
	baytmux "github.com/anawarkar/bay/internal/tmux"
	"github.com/anawarkar/bay/internal/tui/tree"
	"github.com/anawarkar/bay/internal/worktree"
)

type mode int

const (
	modeNormal mode = iota
	modeRename
	modeConfirmDelete
)

// SwitchToCreateMsg tells the app to switch to create screen.
type SwitchToCreateMsg struct {
	PreselectedRepo string
}

// SwitchToSetupMsg tells the app to switch to setup screen.
type SwitchToSetupMsg struct{}

// Model is the sidebar screen state.
type Model struct {
	cfg             *config.Config
	repos           []scanner.Repo
	sessions        []*session.Session
	tree            tree.Model
	mode            mode
	renameInput     textinput.Model
	deleteTarget    string
	activeSession   string
	activeWindowIdx int // tmux window index of the active session
	statusMsg       string
	err             error
	width           int
	height          int
}

// New creates a new sidebar model.
func New(cfg *config.Config) Model {
	ri := textinput.New()
	ri.Placeholder = "new-name"
	ri.CharLimit = 100
	ri.Width = 25

	m := Model{
		cfg:         cfg,
		tree:        tree.New(),
		renameInput: ri,
	}

	// Discover the sidebar pane ID so tmux operations can track it across windows.
	baytmux.InitSidebarPaneID()

	m.refresh()
	return m
}

// refresh reloads repos, sessions, and rebuilds the tree.
func (m *Model) refresh() {
	m.repos = scanner.Scan(m.cfg.ScanDirs)
	m.sessions, _ = session.List()

	// Build session-to-repo mapping
	repoSessions := make(map[string][]*session.Session)
	for _, s := range m.sessions {
		repoSessions[s.Repo] = append(repoSessions[s.Repo], s)
	}

	// Preserve expansion state
	wasExpanded := make(map[string]bool)
	for _, n := range m.tree.Nodes {
		if n.Expanded {
			wasExpanded[n.Name] = true
		}
	}

	// Build tree nodes
	var nodes []tree.Node
	for _, repo := range m.repos {
		node := tree.Node{
			Type:     tree.RepoNode,
			Name:     repo.Name,
			Expanded: wasExpanded[repo.Name],
		}

		sessions := repoSessions[repo.Name]
		if len(sessions) > 0 && !node.Expanded {
			node.Expanded = true
		}

		for _, s := range sessions {
			child := tree.Node{
				Type:        tree.SessionNode,
				Name:        s.Name,
				RepoName:    repo.Name,
				SessionName: s.Name,
				Active:      s.Name == m.activeSession,
			}
			node.Children = append(node.Children, child)
		}

		nodes = append(nodes, node)
	}

	m.tree.SetNodes(nodes)
}

// Init is the Bubbletea init function.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the sidebar.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle rename mode
		if m.mode == modeRename {
			return m.updateRename(msg)
		}
		if m.mode == modeConfirmDelete {
			return m.updateConfirmDelete(msg)
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "k", "up":
			m.tree.MoveUp()
			m.statusMsg = ""
		case "j", "down":
			m.tree.MoveDown()
			m.statusMsg = ""
		case "tab":
			m.tree.Toggle()
		case "enter":
			return m.handleEnter()
		case "n":
			repo := m.tree.SelectedRepoName()
			return m, func() tea.Msg { return SwitchToCreateMsg{PreselectedRepo: repo} }
		case "d":
			return m.startDelete()
		case "r":
			return m.startRename()
		case "c":
			return m.addPane("claude")
		case "s":
			return m, func() tea.Msg { return SwitchToSetupMsg{} }
		}
	}

	return m, nil
}

// switchToSession handles the tmux window management for activating a session.
// If the session has no tmux window yet, one is created. The sidebar pane is moved
// from the current window to the target session's window.
func (m *Model) switchToSession(s *session.Session) error {
	windowIdx := s.TmuxWindow

	// If the session has no window or the window no longer exists, create one
	if windowIdx == 0 || !baytmux.WindowExists(windowIdx) {
		idx, err := baytmux.CreateSessionWindow(s.WorkingDir)
		if err != nil {
			return fmt.Errorf("creating window: %w", err)
		}
		windowIdx = idx
		s.TmuxWindow = idx
		session.Save(s)
	}

	// Move sidebar from current window into the target window
	if err := baytmux.MoveSidebarToWindow(windowIdx); err != nil {
		return fmt.Errorf("moving sidebar: %w", err)
	}

	// Switch to the target window
	if err := baytmux.SwitchToWindow(windowIdx); err != nil {
		return fmt.Errorf("switching window: %w", err)
	}

	// Focus sidebar so user can keep navigating
	baytmux.FocusSidebarPane()

	m.activeSession = s.Name
	m.activeWindowIdx = windowIdx
	return nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	nodeType, _, sessName := m.tree.Selected()
	if nodeType == tree.SessionNode && sessName != "" {
		s, err := session.Load(sessName)
		if err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			return m, nil
		}

		if err := m.switchToSession(s); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			return m, nil
		}

		m.refresh()
		m.statusMsg = fmt.Sprintf("Switched to '%s'", sessName)
		return m, tea.ClearScreen
	} else if nodeType == tree.RepoNode {
		m.tree.Toggle()
	}
	return m, nil
}

func (m Model) startDelete() (tea.Model, tea.Cmd) {
	nodeType, _, sessName := m.tree.Selected()
	if nodeType != tree.SessionNode || sessName == "" {
		m.statusMsg = "Select a session to delete"
		return m, nil
	}
	m.mode = modeConfirmDelete
	m.deleteTarget = sessName
	return m, nil
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeNormal
		s, err := session.Load(m.deleteTarget)
		if err == nil {
			// Kill the session's tmux window if it exists
			if s.TmuxWindow != 0 && baytmux.WindowExists(s.TmuxWindow) {
				baytmux.KillWindow(s.TmuxWindow)
			}
			if s.IsWorktree && s.WorktreeBranch != "" {
				worktree.Remove(s.RepoPath, s.Repo, s.WorktreeBranch)
			}
		}
		session.Delete(m.deleteTarget)
		if m.activeSession == m.deleteTarget {
			m.activeSession = ""
			m.activeWindowIdx = 0
		}
		m.statusMsg = fmt.Sprintf("Deleted '%s'", m.deleteTarget)
		m.deleteTarget = ""
		m.refresh()
		return m, tea.ClearScreen
	case "n", "N", "esc":
		m.mode = modeNormal
		m.deleteTarget = ""
	}
	return m, nil
}

func (m Model) startRename() (tea.Model, tea.Cmd) {
	nodeType, _, sessName := m.tree.Selected()
	if nodeType != tree.SessionNode || sessName == "" {
		m.statusMsg = "Select a session to rename"
		return m, nil
	}
	m.mode = modeRename
	m.renameInput.SetValue(sessName)
	m.renameInput.Focus()
	return m, textinput.Blink
}

func (m Model) updateRename(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		_, _, oldName := m.tree.Selected()
		newName := m.renameInput.Value()
		if newName != "" && newName != oldName {
			if err := session.Rename(oldName, newName); err != nil {
				m.statusMsg = fmt.Sprintf("Rename error: %v", err)
			} else {
				if m.activeSession == oldName {
					m.activeSession = newName
				}
				m.statusMsg = fmt.Sprintf("Renamed to '%s'", newName)
				m.refresh()
			}
		}
		m.mode = modeNormal
		return m, nil
	case "esc":
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

func (m Model) addPane(paneType string) (tea.Model, tea.Cmd) {
	if m.activeSession == "" || m.activeWindowIdx == 0 {
		m.statusMsg = "Activate a session first"
		return m, nil
	}

	s, err := session.Load(m.activeSession)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m, nil
	}

	var command string
	if paneType == "claude" {
		command = "claude"
	}

	if err := baytmux.SplitDevPane(m.activeWindowIdx, s.WorkingDir, command); err != nil {
		m.statusMsg = fmt.Sprintf("Error adding pane: %v", err)
		return m, nil
	}

	// Update session panes
	s.Panes = append(s.Panes, session.Pane{Type: paneType, Cwd: s.WorkingDir})
	session.Save(s)
	m.statusMsg = fmt.Sprintf("Added %s pane", paneType)

	return m, tea.ClearScreen
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

// ActivateSession switches to a session by name — creates/switches to its tmux window.
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

// ViewStatusLine renders just the status/mode line.
func (m Model) viewStatusLine() string {
	if m.mode == modeConfirmDelete {
		return fmt.Sprintf("Delete '%s'? (y/n)", m.deleteTarget)
	}
	if m.mode == modeRename {
		return "Rename: " + m.renameInput.View()
	}
	if m.statusMsg != "" {
		return m.statusMsg
	}
	return ""
}

// HelpLine returns the keybinding help text.
func helpLine() string {
	parts := []string{
		"[n]ew", "[d]el", "[r]en",
		"[t]erm", "[c]laude",
		"[s]etup", "[q]uit",
	}
	return strings.Join(parts, " ")
}
