package create

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"bay/internal/hooks"
	"bay/internal/scanner"
	"bay/internal/session"
	"bay/internal/worktree"
)

type createStep int

const (
	stepPickRepo createStep = iota
	stepWorktreeChoice
	stepBranchName
	stepSessionName
	stepCreating
	stepCreated
)

// DoneMsg signals that session creation is finished.
type DoneMsg struct {
	Session *session.Session
}

// CancelMsg signals the user cancelled the create flow.
type CancelMsg struct{}

// Model is the create session flow state.
type Model struct {
	step         createStep
	repos        []scanner.Repo
	repoCursor   int
	useWorktree  bool
	branchInput  textinput.Model
	nameInput    textinput.Model
	selectedRepo scanner.Repo
	err          error
	created      *session.Session
}

// New creates a new session creation flow.
func New(repos []scanner.Repo, preselectedRepo string) Model {
	bi := textinput.New()
	bi.Placeholder = "feature-branch"
	bi.CharLimit = 100
	bi.Width = 30

	ni := textinput.New()
	ni.Placeholder = "repo-branch"
	ni.CharLimit = 100
	ni.Width = 30

	m := Model{
		step:        stepPickRepo,
		repos:       repos,
		branchInput: bi,
		nameInput:   ni,
	}

	if preselectedRepo != "" {
		for i, r := range repos {
			if r.Name == preselectedRepo {
				m.repoCursor = i
				m.selectedRepo = r
				m.step = stepWorktreeChoice // skip repo picker
				break
			}
		}
	}

	return m
}

// Init starts the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles input for session creation.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		m.step = stepCreated
		return m, nil

	case sessionCreatedMsg:
		m.created = msg.session
		m.step = stepCreated
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m, func() tea.Msg { return CancelMsg{} }
		}

		switch m.step {
		case stepPickRepo:
			switch msg.String() {
			case "j", "down":
				if m.repoCursor < len(m.repos)-1 {
					m.repoCursor++
				}
			case "k", "up":
				if m.repoCursor > 0 {
					m.repoCursor--
				}
			case "enter":
				if len(m.repos) > 0 {
					m.selectedRepo = m.repos[m.repoCursor]
					m.step = stepWorktreeChoice
				}
			}

		case stepWorktreeChoice:
			switch msg.String() {
			case "1", "m":
				m.useWorktree = false
				m.nameInput.SetValue(strings.ToLower(m.selectedRepo.Name) + "-main")
				m.nameInput.Focus()
				m.step = stepSessionName
				return m, textinput.Blink
			case "2", "w":
				m.useWorktree = true
				m.branchInput.Focus()
				m.step = stepBranchName
				return m, textinput.Blink
			}

		case stepBranchName:
			switch msg.String() {
			case "enter":
				branch := m.branchInput.Value()
				if branch == "" {
					return m, nil
				}
				autoName := strings.ToLower(m.selectedRepo.Name) + "-" + branch
				m.nameInput.SetValue(autoName)
				m.nameInput.Focus()
				m.step = stepSessionName
				return m, textinput.Blink
			default:
				var cmd tea.Cmd
				m.branchInput, cmd = m.branchInput.Update(msg)
				return m, cmd
			}

		case stepSessionName:
			switch msg.String() {
			case "enter":
				name := m.nameInput.Value()
				if name == "" {
					return m, nil
				}
				m.step = stepCreating
				return m, m.createSession(name)
			default:
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				return m, cmd
			}

		case stepCreated:
			if msg.String() == "enter" {
				return m, func() tea.Msg { return DoneMsg{Session: m.created} }
			}
		}
	}

	return m, nil
}

func (m *Model) createSession(name string) tea.Cmd {
	return func() tea.Msg {
		workDir := m.selectedRepo.Path
		branch := ""
		isWorktree := false

		if m.useWorktree {
			branch = m.branchInput.Value()
			wtPath, err := worktree.Create(m.selectedRepo.Path, m.selectedRepo.Name, branch)
			if err != nil {
				return errMsg{err}
			}
			workDir = wtPath
			isWorktree = true
		}

		// Save session record
		s := &session.Session{
			Name:           name,
			Repo:           m.selectedRepo.Name,
			RepoPath:       m.selectedRepo.Path,
			WorkingDir:     workDir,
			IsWorktree:     isWorktree,
			WorktreeBranch: branch,
			CreatedAt:      time.Now(),
			Panes: []session.Pane{
				{Type: "shell", Cwd: workDir},
			},
		}
		if err := session.Save(s); err != nil {
			return errMsg{err}
		}

		hooks.OnSessionCreate(s.Name, s.Repo, s.WorkingDir)

		return sessionCreatedMsg{session: s}
	}
}

type errMsg struct{ err error }
type sessionCreatedMsg struct{ session *session.Session }
