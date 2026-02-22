package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	// MainSession is the single tmux session bay uses.
	MainSession = "bay"

	// SidebarWidth is the fixed width of the sidebar in columns.
	SidebarWidth = "35"

	// Prefix kept for legacy/test compatibility.
	Prefix = "bay-"
)

// sidebarPaneID caches the unique tmux pane ID (e.g. %0) of the sidebar pane.
// This stays constant across join-pane / break-pane moves.
var sidebarPaneID string

// run executes a tmux command and returns combined output.
func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// SessionExists checks if a tmux session exists.
func SessionExists(name string) bool {
	_, err := run("has-session", "-t", name)
	return err == nil
}

// CreateMainSession creates the bay session with just the sidebar pane.
// Window 0 runs the sidebar TUI — no dev panes until a session is activated.
func CreateMainSession(sidebarCmd string) error {
	if SessionExists(MainSession) {
		return nil
	}

	if _, err := run("new-session", "-d", "-s", MainSession, sidebarCmd); err != nil {
		return fmt.Errorf("new-session: %w", err)
	}

	// Capture the sidebar's unique pane ID so we can track it across windows.
	id, err := run("display-message", "-t", MainSession+":0.0", "-p", "#{pane_id}")
	if err == nil {
		sidebarPaneID = id
	}

	return nil
}

// InitSidebarPaneID discovers the sidebar pane ID if not already cached.
// Call this from the TUI startup path (inside tmux) so we know which pane is the sidebar.
func InitSidebarPaneID() {
	if sidebarPaneID != "" {
		return
	}
	// The sidebar is always the pane running the bay TUI — which is the current pane.
	id, err := run("display-message", "-p", "#{pane_id}")
	if err == nil {
		sidebarPaneID = id
	}
}

// SidebarPaneTarget returns a target string for the sidebar pane.
func SidebarPaneTarget() string {
	if sidebarPaneID != "" {
		return sidebarPaneID
	}
	return MainSession + ":0.0"
}

// KillMainSession kills the entire bay session.
func KillMainSession() error {
	_, err := run("kill-session", "-t", MainSession)
	return err
}

// CreateSessionWindow creates a new tmux window for a session and returns its index.
// The window starts with a single shell pane in the given directory.
func CreateSessionWindow(dir string) (int, error) {
	// Create window in detached mode, print its index
	out, err := run("new-window", "-t", MainSession, "-d", "-c", dir, "-P", "-F", "#{window_index}")
	if err != nil {
		return 0, fmt.Errorf("new-window: %w", err)
	}
	idx, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("parsing window index: %w", err)
	}
	return idx, nil
}

// WindowExists checks if a window index exists in the bay session.
func WindowExists(windowIndex int) bool {
	_, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex))
	return err == nil
}

// MoveSidebarToWindow breaks the sidebar out of its current window and joins it
// into the target window at the left side, then resizes to SidebarWidth.
func MoveSidebarToWindow(windowIndex int) error {
	target := SidebarPaneTarget()

	// Break sidebar out of its current window (keeps it as a hidden pane)
	run("break-pane", "-t", target, "-d", "-s", target)

	// Join sidebar into the target window's first pane, to the left
	targetPane := fmt.Sprintf("%s:%d.0", MainSession, windowIndex)
	if _, err := run("join-pane", "-hb", "-t", targetPane, "-s", target, "-l", SidebarWidth); err != nil {
		return fmt.Errorf("join-pane: %w", err)
	}

	// Lock sidebar width
	run("resize-pane", "-t", SidebarPaneTarget(), "-x", SidebarWidth)

	return nil
}

// SwitchToWindow selects a window in the bay session.
func SwitchToWindow(windowIndex int) error {
	_, err := run("select-window", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex))
	return err
}

// KillWindow kills a specific tmux window.
func KillWindow(windowIndex int) error {
	_, err := run("kill-window", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex))
	return err
}

// DevPaneCount returns the number of panes in the given window (including sidebar if present).
func DevPaneCount(windowIndex int) int {
	out, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex), "-F", "#{pane_id}")
	if err != nil {
		return 0
	}
	count := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return count
}

// AddDevPane creates a new vertical (side-by-side) pane in the specified window.
// It splits from the rightmost existing pane, then locks the sidebar width.
func AddDevPane(windowIndex int, dir, command string) error {
	count := DevPaneCount(windowIndex)
	var target string

	if count <= 1 {
		// Only sidebar (or empty) — split from pane 0
		target = fmt.Sprintf("%s:%d.0", MainSession, windowIndex)
	} else {
		// Split from the rightmost pane
		target = fmt.Sprintf("%s:%d.%d", MainSession, windowIndex, count-1)
	}

	args := []string{"split-window", "-h", "-t", target, "-c", dir}
	if command != "" {
		args = append(args, command)
	}
	if _, err := run(args...); err != nil {
		return fmt.Errorf("split-window: %w", err)
	}

	// Lock sidebar width
	run("resize-pane", "-t", SidebarPaneTarget(), "-x", SidebarWidth)

	return nil
}

// SplitDevPane splits the dev area for another pane (alias for AddDevPane).
func SplitDevPane(windowIndex int, dir, command string) error {
	return AddDevPane(windowIndex, dir, command)
}

// SendToDevPane sends keys to the first dev pane (pane 1) in the given window.
func SendToDevPane(windowIndex int, keys string) error {
	count := DevPaneCount(windowIndex)
	if count < 2 {
		return fmt.Errorf("no dev pane exists")
	}
	_, err := run("send-keys", "-t", fmt.Sprintf("%s:%d.1", MainSession, windowIndex), keys, "Enter")
	return err
}

// FocusDevPane moves focus to the first dev pane (pane 1) in the given window.
func FocusDevPane(windowIndex int) error {
	count := DevPaneCount(windowIndex)
	if count < 2 {
		return nil
	}
	_, err := run("select-pane", "-t", fmt.Sprintf("%s:%d.1", MainSession, windowIndex))
	return err
}

// FocusSidebarPane moves focus to the sidebar pane.
func FocusSidebarPane() error {
	_, err := run("select-pane", "-t", SidebarPaneTarget())
	return err
}

// BindKeys sets up tmux options for the bay session.
func BindKeys() error {
	// Mouse
	run("set-option", "-g", "mouse", "on")

	// Pane border status
	run("set-option", "-g", "pane-border-status", "top")
	run("set-option", "-g", "pane-border-format", " #{?#{==:#{pane_index},0},bay,#(basename #{pane_current_path})} ")
	run("set-option", "-g", "pane-active-border-style", "fg=#F9FAFB,bold")
	run("set-option", "-g", "pane-border-style", "fg=#4B5563")

	// Status bar styling
	run("set-option", "-g", "status-style", "bg=#1F2937,fg=#9CA3AF")
	run("set-option", "-g", "status-left", " bay ")
	run("set-option", "-g", "status-left-style", "bg=#374151,fg=#06B6D4,bold")
	run("set-option", "-g", "status-right", "")
	run("set-option", "-g", "window-status-current-style", "fg=#06B6D4,bold")
	run("set-option", "-g", "window-status-style", "fg=#6B7280")

	// Hook: re-lock sidebar width whenever a pane is closed.
	// Use the cached pane ID to target sidebar regardless of which window it's in.
	sidebarTarget := SidebarPaneTarget()
	run("set-hook", "-g", "after-kill-pane",
		fmt.Sprintf("run-shell 'tmux resize-pane -t %s -x %s 2>/dev/null || true'", sidebarTarget, SidebarWidth))

	// Backtick as a second prefix
	run("set-option", "-g", "prefix2", "`")
	run("bind-key", "`", "send-keys", "`")

	// Repeat timeout
	run("set-option", "-g", "repeat-time", "1000")

	// Arrow keys for pane navigation
	run("bind-key", "-r", "Left", "select-pane", "-L")
	run("bind-key", "-r", "Right", "select-pane", "-R")
	run("bind-key", "-r", "Up", "select-pane", "-U")
	run("bind-key", "-r", "Down", "select-pane", "-D")

	// d/D for splits — preserve sidebar width. Use pane_id for sidebar target.
	run("bind-key", "-r", "d", "run-shell",
		fmt.Sprintf("tmux split-window -h -c '#{pane_current_path}' && tmux resize-pane -t %s -x %s", sidebarTarget, SidebarWidth))
	run("bind-key", "-r", "D", "run-shell",
		fmt.Sprintf("tmux split-window -v -c '#{pane_current_path}' && tmux resize-pane -t %s -x %s", sidebarTarget, SidebarWidth))

	// w to close pane (won't close sidebar)
	run("bind-key", "-r", "w", "if-shell",
		"[ #{pane_index} -ne 0 ]",
		fmt.Sprintf("run-shell 'tmux kill-pane && tmux resize-pane -t %s -x %s'", sidebarTarget, SidebarWidth))

	// s to toggle focus between sidebar and dev panes
	run("bind-key", "-r", "s", "if-shell",
		"[ #{pane_index} -eq 0 ]",
		"select-pane -R",
		fmt.Sprintf("select-pane -t %s", sidebarTarget))

	return nil
}

// ListBaySessions is kept for compatibility.
func ListBaySessions() ([]string, error) {
	return nil, nil
}

// CurrentSession returns the name of the active tmux session.
func CurrentSession() (string, error) {
	out, err := run("display-message", "-p", "#{session_name}")
	if err != nil {
		return "", fmt.Errorf("not in a tmux session")
	}
	return out, nil
}
