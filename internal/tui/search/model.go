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
	"bay/internal/memory"
	"bay/internal/session"
)

// DoneMsg signals a session was selected.
type DoneMsg struct{ SessionName string }

// CancelMsg signals the user cancelled.
type CancelMsg struct{}

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
	PaneInfo    string
}

// Model is the search screen state.
type Model struct {
	input       textinput.Model
	allSessions []enrichedSession
	filtered    []enrichedSession
	cursor      int
	width       int
	height      int
}

// New creates a search model with all session data pre-loaded.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "search sessions..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	sessions, _ := session.List()
	heartbeats := loadHeartbeats()

	var enriched []enrichedSession
	for _, s := range sessions {
		e := enrichedSession{
			Session: s,
			Branch:  s.WorktreeBranch,
			Note:    s.Note,
		}

		// Working state for extra context
		if w, err := memory.GetWorking(s.Name); err == nil && w != nil {
			if e.Branch == "" {
				e.Branch = w.GitBranch
			}
		}

		// Diff
		if s.WorkingDir != "" {
			e.Diff = computeDiff(s.WorkingDir)
		}

		// Agent info
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
		if e.HasAgent {
			if t, ok := heartbeats[s.Name]; ok && time.Since(t) < constants.AgentIdleThreshold {
				e.AgentActive = true
			}
		}

		// Pane info string
		var parts []string
		if agents > 0 {
			label := fmt.Sprintf("%d agent", agents)
			if e.AgentActive {
				label += " (active)"
			} else if e.HasAgent {
				label += " (idle)"
			}
			parts = append(parts, label)
		}
		if shells > 0 {
			parts = append(parts, fmt.Sprintf("%d shell", shells))
		}
		e.PaneInfo = strings.Join(parts, " · ")

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

	m := Model{
		input:       ti,
		allSessions: enriched,
		filtered:    enriched,
	}
	return m
}

// Init starts the blinking cursor.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, func() tea.Msg { return CancelMsg{} }
		case "enter":
			if m.cursor < len(m.filtered) {
				return m, func() tea.Msg {
					return DoneMsg{SessionName: m.filtered[m.cursor].Session.Name}
				}
			}
			return m, nil
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}

		// Forward to text input
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.filter()
		return m, cmd
	}

	return m, nil
}

func (m *Model) filter() {
	query := strings.ToLower(m.input.Value())
	if query == "" {
		m.filtered = m.allSessions
		m.cursor = 0
		return
	}

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
