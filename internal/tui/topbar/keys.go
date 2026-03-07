package topbar

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	// Always available
	Tab         key.Binding
	RepoNext    key.Binding
	FocusToggle key.Binding

	// Focused mode only
	Left   key.Binding
	Right  key.Binding
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	New    key.Binding
	Memory key.Binding
	Delete key.Binding
	Rename key.Binding
	Quit   key.Binding
	Escape key.Binding
}

var keys = keyMap{
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "cycle session"),
	),
	RepoNext: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "next repo"),
	),
	FocusToggle: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("`+space", "focus bar"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "repos row"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "sessions row"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "activate"),
	),
	New: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new session"),
	),
	Memory: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "memory"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Rename: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "rename"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit bay"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "unfocus"),
	),
}
