package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// MainSession is the single tmux session bay uses.
	MainSession = "bay"

	// TopbarHeight is the fixed height of the topbar in lines.
	TopbarHeight = "5"

	// Prefix kept for legacy/test compatibility.
	Prefix = "bay-"
)

// topbarPaneID caches the unique tmux pane ID (e.g. %0) of the topbar pane.
// This stays constant across join-pane / break-pane moves.
var topbarPaneID string

// topbarPaneFile returns the path where the topbar pane ID is persisted.
func topbarPaneFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bay", ".topbar-pane-id")
}

// saveTopbarPaneID writes the pane ID to disk so future processes can find it.
func saveTopbarPaneID(id string) {
	f := topbarPaneFile()
	os.MkdirAll(filepath.Dir(f), 0o755)
	os.WriteFile(f, []byte(strings.TrimSpace(id)), 0o644)
}

// loadTopbarPaneID reads the persisted pane ID from disk.
func loadTopbarPaneID() string {
	b, err := os.ReadFile(topbarPaneFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// paneExists returns true if the given tmux pane ID is alive in the bay session.
func paneExists(paneID string) bool {
	if paneID == "" {
		return false
	}
	_, err := run("display-message", "-t", paneID, "-p", "#{pane_id}")
	return err == nil
}

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

// wrapTopbarCmd wraps the topbar command so it auto-restarts on unexpected exit.
// A normal exit (status 0, from bay quitting) breaks the loop.
func wrapTopbarCmd(cmd string) string {
	return fmt.Sprintf("bash -c 'while true; do %s; [ $? -eq 0 ] && break; sleep 0.2; done'", cmd)
}

// CreateMainSession creates the bay session with just the topbar pane.
// If the session already exists and the topbar is still alive, leaves it alone.
// If the topbar is gone, spawns a new one.
func CreateMainSession(topbarCmd string) error {
	wrapped := wrapTopbarCmd(topbarCmd)

	if SessionExists(MainSession) {
		// Use the persisted pane ID to find the topbar without guessing by position.
		savedID := loadTopbarPaneID()
		if paneExists(savedID) {
			// Topbar is alive — nothing to do.
			topbarPaneID = savedID
			return nil
		}
		// Topbar is gone — spawn a fresh one in a new window.
		out, err := run("new-window", "-t", MainSession+":", "-d", "-P", "-F", "#{pane_id}", "--", "bash", "-c", wrapped)
		if err != nil {
			return fmt.Errorf("new-window for topbar: %w", err)
		}
		topbarPaneID = strings.TrimSpace(out)
		saveTopbarPaneID(topbarPaneID)
		return nil
	}

	if _, err := run("new-session", "-d", "-s", MainSession, wrapped); err != nil {
		return fmt.Errorf("new-session: %w", err)
	}

	// Capture the topbar's unique pane ID so we can track it across windows.
	id, err := run("display-message", "-t", MainSession+":0.0", "-p", "#{pane_id}")
	if err == nil {
		topbarPaneID = strings.TrimSpace(id)
		saveTopbarPaneID(topbarPaneID)
	}

	return nil
}

// InitTopbarPaneID discovers the topbar pane ID if not already cached.
// Call this from the TUI startup path (inside tmux) so we know which pane is the topbar.
func InitTopbarPaneID() {
	if topbarPaneID != "" {
		return
	}
	// $TMUX_PANE is set by tmux to the ID of the pane this process is running in.
	// This is reliable regardless of where the client is currently focused.
	// display-message without -t would return the CLIENT'S active pane, not ours.
	if id := os.Getenv("TMUX_PANE"); id != "" {
		topbarPaneID = strings.TrimSpace(id)
		saveTopbarPaneID(topbarPaneID)
		return
	}
	// Fallback (should not be reached when running inside tmux).
	id, err := run("display-message", "-t", os.Getenv("TMUX_PANE"), "-p", "#{pane_id}")
	if err == nil {
		topbarPaneID = strings.TrimSpace(id)
		saveTopbarPaneID(topbarPaneID)
	}
}

// TopbarPaneTarget returns a target string for the topbar pane.
func TopbarPaneTarget() string {
	if topbarPaneID != "" {
		return topbarPaneID
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
	out, err := run("new-window", "-t", MainSession+":", "-d", "-c", dir, "-P", "-F", "#{window_index}")
	if err != nil {
		return 0, fmt.Errorf("new-window: %w (tmux: %s)", err, out)
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

// MoveTopbarToWindow moves the topbar pane into the target window at the top.
func MoveTopbarToWindow(windowIndex int) error {
	target := TopbarPaneTarget()

	// Check if topbar is already in the target window — skip move if so.
	currentWindow, err := run("display-message", "-t", target, "-p", "#{window_index}")
	if err == nil && strings.TrimSpace(currentWindow) == fmt.Sprintf("%d", windowIndex) {
		run("resize-pane", "-t", target, "-y", TopbarHeight)
		return nil
	}

	targetPane := fmt.Sprintf("%s:%d.0", MainSession, windowIndex)

	// move-pane atomically moves the pane to the target window above the first pane.
	if out, err := run("move-pane", "-vb", "-s", target, "-t", targetPane, "-l", TopbarHeight); err != nil {
		return fmt.Errorf("move-pane: %w (tmux: %s)", err, out)
	}

	// Lock topbar height
	run("resize-pane", "-t", TopbarPaneTarget(), "-y", TopbarHeight)

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

// BreakTopbarToOwnWindow moves the topbar pane into its own detached window.
// Returns the new window index, or -1 on error.
// Call this before killing a window that currently contains the topbar pane.
func BreakTopbarToOwnWindow() int {
	out, err := run("break-pane", "-d", "-s", TopbarPaneTarget(), "-P", "-F", "#{window_index}")
	if err != nil {
		return -1
	}
	idx, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return -1
	}
	return idx
}

// SelectWindow switches the tmux client to the given window index.
func SelectWindow(windowIndex int) {
	run("select-window", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex))
}

// DevPaneCount returns the number of panes in the given window (including topbar if present).
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
// It splits from the rightmost existing pane, then locks the topbar height.
func AddDevPane(windowIndex int, dir, command string) error {
	count := DevPaneCount(windowIndex)
	var target string

	if count <= 1 {
		// Only topbar (or empty) — split from pane 0
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

	// Lock topbar height
	run("resize-pane", "-t", TopbarPaneTarget(), "-y", TopbarHeight)

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

// FocusTopbarPane moves focus to the topbar pane.
func FocusTopbarPane() error {
	_, err := run("select-pane", "-t", TopbarPaneTarget())
	return err
}

// FocusBelowTopbar moves focus to the pane below the topbar.
func FocusBelowTopbar() error {
	_, err := run("select-pane", "-D")
	return err
}

// RunnerFunc is a function that executes tmux commands. Used for testing.
type RunnerFunc func(args ...string) (string, error)

// BindKeysWithRunner sets up tmux options using the provided runner function.
// This allows testing without a real tmux server.
func BindKeysWithRunner(runner RunnerFunc) error {
	return bindKeysImpl(runner)
}

// BindKeys sets up tmux options for the bay session.
func BindKeys() error {
	return bindKeysImpl(run)
}

func bindKeysImpl(run RunnerFunc) error {
	// Mouse
	run("set-option", "-g", "mouse", "on")
	// Prevent clicking window names in the status bar from switching windows.
	run("unbind-key", "-n", "MouseDown1Status")
	run("unbind-key", "-n", "MouseDown1StatusLeft")
	run("unbind-key", "-n", "MouseDown1StatusRight")

	// Pane border status
	run("set-option", "-g", "pane-border-status", "top")
	run("set-option", "-g", "pane-border-format", " #{?#{==:#{pane_index},0},bay,#(basename #{pane_current_path})} ")
	run("set-option", "-g", "pane-active-border-style", "fg=#F9FAFB,bold")
	run("set-option", "-g", "pane-border-style", "fg=#4B5563")

	// Status bar styling
	run("set-option", "-g", "status-style", "bg=#1F2937,fg=#9CA3AF")
	run("set-option", "-g", "status-left", " #{?client_prefix,⌘ CMD, bay} ")
	run("set-option", "-g", "status-left-style", "#{?client_prefix,bg=#7C3AED fg=#F9FAFB bold,bg=#374151 fg=#06B6D4 bold}")
	run("set-option", "-g", "status-right", "")
	run("set-option", "-g", "window-status-current-style", "fg=#06B6D4,bold")
	run("set-option", "-g", "window-status-style", "fg=#6B7280")
	// Hide window list from status bar — the topbar IS the session switcher.
	run("set-option", "-g", "window-status-format", "")
	run("set-option", "-g", "window-status-current-format", "")

	// Shell snippet that resolves the topbar pane ID at runtime from the persisted file.
	// This stays correct even after the topbar moves between windows.
	topbarIDFile := "$HOME/.bay/.topbar-pane-id"
	resizeTopbar := fmt.Sprintf("tmux resize-pane -t $(cat %s) -y %s 2>/dev/null || true", topbarIDFile, TopbarHeight)

	// Hook: re-lock topbar height whenever a pane is closed.
	run("set-hook", "-g", "after-kill-pane",
		fmt.Sprintf("run-shell '%s'", resizeTopbar))

	// Backtick as a second prefix
	run("set-option", "-g", "prefix2", "`")
	run("bind-key", "`", "send-keys", "`")

	// Unbind native window-switching keys — all switching goes through topbar.
	run("unbind-key", "n") // next-window
	run("unbind-key", "p") // previous-window
	run("unbind-key", "l") // last-window
	for i := 0; i <= 9; i++ {
		// Already rebound on prefix2 (backtick) to send-keys to topbar.
		// Unbind on primary prefix (Ctrl+B) to prevent bypass.
		run("unbind-key", "-T", "prefix", fmt.Sprintf("%d", i))
	}

	// Repeat timeout
	run("set-option", "-g", "repeat-time", "1000")

	// Arrow keys for pane navigation
	run("bind-key", "-r", "Left", "select-pane", "-L")
	run("bind-key", "-r", "Right", "select-pane", "-R")
	run("bind-key", "-r", "Up", "select-pane", "-U")
	run("bind-key", "-r", "Down", "select-pane", "-D")

	// d/D for splits — read topbar pane ID at runtime so it works after moves.
	run("bind-key", "-r", "d", "run-shell",
		fmt.Sprintf("tmux split-window -h -c '#{pane_current_path}' && %s", resizeTopbar))
	run("bind-key", "-r", "D", "run-shell",
		fmt.Sprintf("tmux split-window -v -c '#{pane_current_path}' && %s", resizeTopbar))

	// a for agent split — vertical split running claude in same dir
	// Uses bash -c so Claude Code's SessionStart hook (bay context) fires correctly.
	run("bind-key", "-r", "a", "run-shell",
		fmt.Sprintf("tmux split-window -h -c '#{pane_current_path}' 'bash -c \"claude\"' && %s", resizeTopbar))

	// w to close pane — guard by comparing pane_id against topbar's persisted ID.
	run("bind-key", "-r", "w", "if-shell",
		fmt.Sprintf("[ \"#{pane_id}\" != \"$(cat %s)\" ]", topbarIDFile),
		fmt.Sprintf("run-shell 'tmux kill-pane && %s'", resizeTopbar))

	// s to toggle focus between topbar and dev panes
	run("bind-key", "-r", "s", "if-shell",
		"[ #{pane_index} -eq 0 ]",
		"select-pane -D",
		"select-pane -t .0")

	// Quick-access keybinds: send keys to topbar pane (pane 0 of current window)
	// Use .0 to dynamically target pane index 0 in the active window.
	// `+Space → focus topbar and toggle focused mode
	run("bind-key", "Space", "run-shell",
		"tmux send-keys -t .0 q; tmux select-pane -t .0")

	// `+Tab → cycle session (repeatable: `+Tab+Tab+Tab...)
	run("bind-key", "-r", "Tab", "send-keys", "-t", ".0", "Tab")

	// `+r → cycle repo
	run("bind-key", "r", "send-keys", "-t", ".0", "r")

	// `+0-9 → jump to session by index
	for i := 0; i <= 9; i++ {
		key := fmt.Sprintf("%d", i)
		run("bind-key", key, "send-keys", "-t", ".0", key)
	}

	// Memory hooks: capture pane buffer on pane exit for episodic recording
	run("set-hook", "-g", "pane-exited",
		"run-shell 'bay mem capture #{pane_id} &'")

	return nil
}

// CapturePaneBuffer captures the last N lines of a pane's scrollback.
func CapturePaneBuffer(paneID string, lines int) (string, error) {
	startLine := fmt.Sprintf("-%d", lines)
	out, err := run("capture-pane", "-p", "-t", paneID, "-S", startLine)
	if err != nil {
		return "", fmt.Errorf("capture-pane %s: %w", paneID, err)
	}
	return out, nil
}

// CaptureAllDevPanes captures all non-topbar pane buffers in a window.
func CaptureAllDevPanes(windowIndex, lines int) (map[string]string, error) {
	// List all panes in the window
	out, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex), "-F", "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("list-panes: %w", err)
	}

	topbarID := TopbarPaneTarget()
	result := make(map[string]string)

	for _, line := range strings.Split(out, "\n") {
		paneID := strings.TrimSpace(line)
		if paneID == "" || paneID == topbarID {
			continue
		}
		buffer, err := CapturePaneBuffer(paneID, lines)
		if err != nil {
			continue
		}
		result[paneID] = buffer
	}

	return result, nil
}

// PaneInfo holds information about a single tmux pane.
type PaneInfo struct {
	PaneID  string
	Command string
	Cwd     string
	IsAgent bool
}

// SnapshotPaneLayout queries tmux for the current pane layout of a window.
// Returns all panes except the topbar pane.
func SnapshotPaneLayout(windowIndex int) ([]PaneInfo, error) {
	out, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex),
		"-F", "#{pane_id} #{pane_start_command} #{pane_current_path}")
	if err != nil {
		return nil, fmt.Errorf("list-panes: %w", err)
	}

	topbarID := TopbarPaneTarget()
	var panes []PaneInfo

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}

		paneID := parts[0]
		startCmd := parts[1]
		cwd := parts[2]

		// Skip topbar pane
		if paneID == topbarID {
			continue
		}

		// Detect agent panes by checking if the start command contains "claude"
		isAgent := strings.Contains(startCmd, "claude")
		panes = append(panes, PaneInfo{
			PaneID:  paneID,
			Command: startCmd,
			Cwd:     cwd,
			IsAgent: isAgent,
		})
	}

	return panes, nil
}

// RecreateSessionPanes recreates additional panes in a window from saved layout.
// The first pane is already created by CreateSessionWindow — this handles the rest.
func RecreateSessionPanes(windowIndex int, panes []SessionPane) error {
	for _, p := range panes {
		dir := p.Cwd
		if dir == "" {
			continue
		}

		var command string
		if p.Type == "agent" {
			// Fresh Claude — SessionStart hook provides context injection
			command = "bash -c \"claude\""
		}

		args := []string{"split-window", "-h", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex), "-c", dir}
		if command != "" {
			args = append(args, command)
		}
		if _, err := run(args...); err != nil {
			continue // Non-fatal: best-effort recreation
		}
	}

	// Lock topbar height after adding panes
	run("resize-pane", "-t", TopbarPaneTarget(), "-y", TopbarHeight)

	return nil
}

// SessionPane is a minimal pane description used for recreation.
// This mirrors session.Pane but avoids the import cycle.
type SessionPane struct {
	Type    string
	Cwd     string
	Command string
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
