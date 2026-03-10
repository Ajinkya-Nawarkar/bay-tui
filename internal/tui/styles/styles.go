package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7C3AED") // purple
	Secondary = lipgloss.Color("#06B6D4") // cyan
	Muted     = lipgloss.Color("#6B7280") // gray
	Success   = lipgloss.Color("#10B981") // green
	Focus     = lipgloss.Color("#FBBF24") // yellow — focus cursor
	Danger    = lipgloss.Color("#EF4444") // red
	White     = lipgloss.Color("#F9FAFB")
	Dim       = lipgloss.Color("#4B5563")
	BgDark    = lipgloss.Color("#1F2937")

	// Title
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary)

	Subtitle = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)

	// Box for sections
	SectionBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Dim).
			Padding(0, 1)

	ActiveSectionBox = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Primary).
				Padding(0, 1)

	// Tree styles
	RepoName = lipgloss.NewStyle().
			Bold(true).
			Foreground(Secondary)

	SessionName = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	ActiveSession = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	SelectedLine = lipgloss.NewStyle().
			Background(lipgloss.Color("#374151")).
			Foreground(White).
			Bold(true)

	NoSessions = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// Note text (orange)
	NoteText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F97316"))

	// Help bar
	HelpBar = lipgloss.NewStyle().
		Foreground(Dim)

	HelpKey = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true)

	// Status
	StatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FCD34D")).
			Bold(true)

	// Input styles
	Prompt = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	InputLabel = lipgloss.NewStyle().
			Foreground(White).
			MarginBottom(1)

	ErrorText = lipgloss.NewStyle().
			Foreground(Danger).
			Bold(true)

	SuccessText = lipgloss.NewStyle().
			Foreground(Success)

	// Topbar styles
	FocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Focus).
			Padding(0, 1)

	RepoTab = lipgloss.NewStyle().
		Foreground(Muted)

	RepoTabActive = lipgloss.NewStyle().
			Foreground(Secondary). // cyan — currently active repo
			Bold(true)

	RepoTabFocused = lipgloss.NewStyle().
			Foreground(Focus). // yellow — focus cursor on repo row
			Bold(true)

	SessionTab = lipgloss.NewStyle().
			Foreground(Muted)

	SessionTabActive = lipgloss.NewStyle().
				Foreground(Success). // green — currently active session
				Bold(true)

	SessionTabFocused = lipgloss.NewStyle().
				Foreground(Focus). // yellow — focus cursor on session row
				Bold(true)

	SessionTabStale = lipgloss.NewStyle().
			Foreground(Dim). // dimmed — working dir missing
			Strikethrough(true)

	// Layout
	AppContainer = lipgloss.NewStyle().
			Padding(0, 1)
)
