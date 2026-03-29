package search

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

	"bay/internal/constants"
	"bay/internal/session"
)

// DoneMsg signals a session was selected.
type DoneMsg struct{ SessionName string }

// CancelMsg signals the user cancelled.
type CancelMsg struct{}

type refreshMsg struct{}

type activityState int

const (
	stateActive activityState = iota
	stateIdle
	stateDormant
)

type diffSummary struct {
	Files      int
	Insertions int
	Deletions  int
	Clean      bool
}

type enrichedSession struct {
	Session     *session.Session
	Branch      string
	Note        string
	Diff        diffSummary
	HasAgent    bool
	AgentActive bool
	Heartbeat   time.Time
	State       activityState
	PaneInfo    string
}

type section struct {
	Label    string
	Sessions []enrichedSession
}

// Model is the combined search + status screen state.
type Model struct {
	input       textinput.Model
	allSessions []enrichedSession
	filtered    []enrichedSession // used when query is active
	sections    []section         // used when query is empty (grouped view)
	cursor      int
	width       int
	height      int
	summary     string
}

// IsSearching returns true when a query is active (flat filtered list).
func (m Model) IsSearching() bool {
	return m.input.Value() != ""
}

// New creates a search model with all session data pre-loaded.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "type to search, or browse sessions below..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	m := Model{input: ti}
	m.loadData()
	return m
}

// Init starts the blinking cursor and heartbeat refresh ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.Tick(constants.StatusRefreshInterval, func(time.Time) tea.Msg {
			return refreshMsg{}
		}),
	)
}

// Update handles input and refresh ticks.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case refreshMsg:
		m.refreshHeartbeats()
		return m, tea.Tick(constants.StatusRefreshInterval, func(time.Time) tea.Msg {
			return refreshMsg{}
		})

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, func() tea.Msg { return CancelMsg{} }
		case "enter":
			list := m.visibleList()
			if m.cursor < len(list) {
				return m, func() tea.Msg {
					return DoneMsg{SessionName: list[m.cursor].Session.Name}
				}
			}
			return m, nil
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j", "ctrl+n":
			list := m.visibleList()
			if m.cursor < len(list)-1 {
				m.cursor++
			}
			return m, nil
		case "ctrl+r":
			m.loadData()
			return m, nil
		}

		// Forward to text input
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.rebuildView()
		return m, cmd
	}

	return m, nil
}

// visibleList returns the flat list of sessions the cursor navigates over.
func (m Model) visibleList() []enrichedSession {
	if m.IsSearching() {
		return m.filtered
	}
	// Flattened from sections
	var flat []enrichedSession
	for _, sec := range m.sections {
		flat = append(flat, sec.Sessions...)
	}
	return flat
}

func (m *Model) rebuildView() {
	if m.IsSearching() {
		m.filterByQuery()
	} else {
		m.buildSections()
	}
}

func (m *Model) filterByQuery() {
	query := strings.ToLower(m.input.Value())
	var matches []enrichedSession
	for _, e := range m.allSessions {
		label := strings.ToLower(e.Session.Repo + "/" + e.Session.Name)
		branch := strings.ToLower(e.Branch)
		if strings.Contains(label, query) || strings.Contains(branch, query) {
			matches = append(matches, e)
		}
	}
	m.filtered = matches
	m.cursor = 0
}

func (m *Model) buildSections() {
	groups := map[activityState][]enrichedSession{}
	for _, e := range m.allSessions {
		groups[e.State] = append(groups[e.State], e)
	}

	m.sections = nil
	for _, state := range []activityState{stateActive, stateIdle, stateDormant} {
		if len(groups[state]) == 0 {
			continue
		}
		label := "DORMANT"
		switch state {
		case stateActive:
			label = "ACTIVE"
		case stateIdle:
			label = "IDLE"
		}
		m.sections = append(m.sections, section{
			Label:    label,
			Sessions: groups[state],
		})
	}

	// Clamp cursor
	flat := m.visibleList()
	if m.cursor >= len(flat) {
		m.cursor = len(flat) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// Summary
	active := len(groups[stateActive])
	idle := len(groups[stateIdle])
	dormant := len(groups[stateDormant])
	m.summary = fmt.Sprintf("%d active \u00b7 %d idle \u00b7 %d dormant", active, idle, dormant)
}

func (m *Model) loadData() {
	sessions, _ := session.List()
	heartbeats := loadHeartbeats()

	var enriched []enrichedSession
	for _, s := range sessions {
		e := enrichSession(s, heartbeats)
		enriched = append(enriched, e)
	}

	// Sort MRU
	sort.Slice(enriched, func(i, j int) bool {
		ti := enriched[i].Session.LastActiveAt
		if ti.IsZero() {
			ti = enriched[i].Session.CreatedAt
		}
		tj := enriched[j].Session.LastActiveAt
		if tj.IsZero() {
			tj = enriched[j].Session.CreatedAt
		}
		return ti.After(tj)
	})

	m.allSessions = enriched
	m.rebuildView()
}

func (m *Model) refreshHeartbeats() {
	heartbeats := loadHeartbeats()
	changed := false
	for i := range m.allSessions {
		e := &m.allSessions[i]
		if t, ok := heartbeats[e.Session.Name]; ok {
			e.Heartbeat = t
		}
		newActive := e.HasAgent && !e.Heartbeat.IsZero() && time.Since(e.Heartbeat) < constants.AgentIdleThreshold
		if newActive != e.AgentActive {
			e.AgentActive = newActive
			changed = true
		}
		newState := classify(*e)
		if newState != e.State {
			e.State = newState
			changed = true
		}
	}
	if changed {
		m.rebuildView()
	}
}

func enrichSession(s *session.Session, heartbeats map[string]time.Time) enrichedSession {
	e := enrichedSession{
		Session: s,
		Branch:  s.WorktreeBranch,
		Note:    s.Purpose,
	}

	if s.WorkingDir != "" {
		e.Diff = computeDiff(s.WorkingDir)
	}

	shells, agents := 0, 0
	for _, p := range s.Panes {
		if p.Type == "agent" {
			agents++
			e.HasAgent = true
		} else {
			shells++
		}
	}
	if !e.HasAgent {
		if _, ok := heartbeats[s.Name]; ok {
			e.HasAgent = true
			agents = 1
		}
	}
	if t, ok := heartbeats[s.Name]; ok {
		e.Heartbeat = t
		if e.HasAgent && time.Since(t) < constants.AgentIdleThreshold {
			e.AgentActive = true
		}
	}

	e.State = classify(e)

	// Pane info string
	var parts []string
	if agents > 0 {
		label := fmt.Sprintf("%d agent", agents)
		switch e.State {
		case stateActive:
			label += fmt.Sprintf(" (active, %s ago)", relativeTime(e.Heartbeat))
		case stateIdle:
			label += fmt.Sprintf(" (idle %s)", relativeTime(e.Heartbeat))
		}
		parts = append(parts, label)
	}
	if shells > 0 {
		parts = append(parts, fmt.Sprintf("%d shell", shells))
	}
	e.PaneInfo = strings.Join(parts, " · ")

	return e
}

func classify(e enrichedSession) activityState {
	if !e.HasAgent {
		return stateDormant
	}
	if e.Heartbeat.IsZero() {
		return stateDormant
	}
	elapsed := time.Since(e.Heartbeat)
	if elapsed < constants.AgentIdleThreshold {
		return stateActive
	}
	if elapsed < constants.AgentDormantThreshold {
		return stateIdle
	}
	return stateDormant
}

// --- Data helpers ---

func computeDiff(workDir string) diffSummary {
	out1, _ := exec.Command("git", "-C", workDir, "diff", "--shortstat").Output()
	f1, i1, d1 := parseShortstat(string(out1))

	out2, _ := exec.Command("git", "-C", workDir, "diff", "--cached", "--shortstat").Output()
	f2, i2, d2 := parseShortstat(string(out2))

	files := f1 + f2
	ins := i1 + i2
	del := d1 + d2

	var untracked int
	out3, _ := exec.Command("git", "-C", workDir, "status", "--porcelain").Output()
	for _, line := range strings.Split(string(out3), "\n") {
		if len(line) >= 2 && line[:2] == "??" {
			untracked++
		}
	}

	return diffSummary{
		Files:      files,
		Insertions: ins,
		Deletions:  del,
		Clean:      files == 0 && ins == 0 && del == 0 && untracked == 0,
	}
}

var shortstatRe = regexp.MustCompile(`(\d+) file`)
var insertionsRe = regexp.MustCompile(`(\d+) insertion`)
var deletionsRe = regexp.MustCompile(`(\d+) deletion`)

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

func loadHeartbeats() map[string]time.Time {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".bay", "agent-status")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
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
	return heartbeats
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
