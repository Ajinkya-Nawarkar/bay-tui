package status

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

	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/constants"
	"bay/internal/memory"
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
	ComputedAt time.Time
}

type statusSession struct {
	Session   *session.Session
	Branch    string
	Note      string
	Diff      diffSummary
	HasAgent  bool
	Heartbeat time.Time
	State     activityState
	PaneInfo  string
}

type section struct {
	State    activityState
	Label    string
	Sessions []statusSession
}

// Model is the status dashboard screen state.
type Model struct {
	sections []section
	flatList []statusSession
	cursor   int
	width    int
	height   int
	summary  string
}

// New creates a status model with all session data pre-loaded.
func New() Model {
	m := Model{}
	m.loadData()
	return m
}

// Init starts the refresh ticker.
func (m Model) Init() tea.Cmd {
	return tea.Tick(constants.StatusRefreshInterval, func(time.Time) tea.Msg {
		return refreshMsg{}
	})
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
		case "esc", "q", "ctrl+c":
			return m, func() tea.Msg { return CancelMsg{} }
		case "enter":
			if m.cursor < len(m.flatList) {
				return m, func() tea.Msg {
					return DoneMsg{SessionName: m.flatList[m.cursor].Session.Name}
				}
			}
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.flatList)-1 {
				m.cursor++
			}
			return m, nil
		case "r":
			m.loadData()
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) loadData() {
	sessions, _ := session.List()
	heartbeats := loadHeartbeats()

	var all []statusSession
	for _, s := range sessions {
		ss := statusSession{
			Session: s,
			Branch:  s.WorktreeBranch,
			Note:    s.Note,
		}

		if w, err := memory.GetWorking(s.Name); err == nil && w != nil {
			if ss.Branch == "" {
				ss.Branch = w.GitBranch
			}
		}

		if s.WorkingDir != "" {
			ss.Diff = computeDiff(s.WorkingDir)
		}

		// Agent detection
		shells, agents := 0, 0
		for _, p := range s.Panes {
			if p.Type == "agent" {
				agents++
				ss.HasAgent = true
			} else {
				shells++
			}
		}
		if !ss.HasAgent {
			if _, ok := heartbeats[s.Name]; ok {
				ss.HasAgent = true
				agents = 1
			}
		}
		if t, ok := heartbeats[s.Name]; ok {
			ss.Heartbeat = t
		}

		// Classify
		ss.State = classify(ss)

		// Pane info
		var parts []string
		if agents > 0 {
			label := fmt.Sprintf("%d agent", agents)
			switch ss.State {
			case stateActive:
				label += fmt.Sprintf(" (active, %s ago)", relativeTime(ss.Heartbeat))
			case stateIdle:
				label += fmt.Sprintf(" (idle %s)", relativeTime(ss.Heartbeat))
			}
			parts = append(parts, label)
		}
		if shells > 0 {
			parts = append(parts, fmt.Sprintf("%d shell", shells))
		}
		ss.PaneInfo = strings.Join(parts, " · ")

		all = append(all, ss)
	}

	m.buildSections(all)
}

func (m *Model) refreshHeartbeats() {
	heartbeats := loadHeartbeats()
	changed := false
	for i := range m.flatList {
		ss := &m.flatList[i]
		if t, ok := heartbeats[ss.Session.Name]; ok {
			ss.Heartbeat = t
		}
		newState := classify(*ss)
		if newState != ss.State {
			ss.State = newState
			changed = true
		}
	}
	if changed {
		m.buildSections(m.flatList)
	}
}

func (m *Model) buildSections(all []statusSession) {
	// Sort within each group by last activity
	sort.Slice(all, func(i, j int) bool {
		ti := all[i].Session.LastActiveAt
		if ti.IsZero() {
			ti = all[i].Session.CreatedAt
		}
		tj := all[j].Session.LastActiveAt
		if tj.IsZero() {
			tj = all[j].Session.CreatedAt
		}
		return ti.After(tj)
	})

	groups := map[activityState][]statusSession{}
	for _, ss := range all {
		groups[ss.State] = append(groups[ss.State], ss)
	}

	m.sections = nil
	m.flatList = nil
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
			State:    state,
			Label:    label,
			Sessions: groups[state],
		})
		m.flatList = append(m.flatList, groups[state]...)
	}

	// Clamp cursor
	if m.cursor >= len(m.flatList) {
		m.cursor = len(m.flatList) - 1
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

func classify(ss statusSession) activityState {
	if !ss.HasAgent {
		return stateDormant
	}
	if ss.Heartbeat.IsZero() {
		return stateDormant
	}
	elapsed := time.Since(ss.Heartbeat)
	if elapsed < constants.AgentIdleThreshold {
		return stateActive
	}
	if elapsed < constants.AgentDormantThreshold {
		return stateIdle
	}
	return stateDormant
}

// --- Data helpers (same as search package) ---

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
		ComputedAt: time.Now(),
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
