package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7C3AED") // purple
	Secondary = lipgloss.Color("#06B6D4") // cyan
	Muted     = lipgloss.Color("#6B7280") // gray
	Success   = lipgloss.Color("#10B981") // green
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

	// Help bar
	HelpBar = lipgloss.NewStyle().
		Foreground(Muted)

	HelpKey = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true)

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

	// Layout — no padding, sidebar manages its own layout
	AppContainer = lipgloss.NewStyle().
			Padding(0, 1)
)
