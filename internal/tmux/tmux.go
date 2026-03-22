// Package tmux wraps tmux CLI commands for bay's single-session architecture.
//
// All sessions share one tmux session named "bay"; each bay session owns one window.
// Topbar pane is physically moved between windows via join-pane/break-pane.
// Topbar pane ID persisted to ~/.bay/.topbar-pane-id for stability across moves.
// bindKeys sets up prefix2 (backtick), hooks, and keybindings for the bay session.
// wrapTopbarCmd generates a restart loop so the TUI auto-recovers from crashes.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"bay/internal/constants"
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

// collectDiagnostics gathers tmux environment info for debugging startup failures.
func collectDiagnostics() string {
	ver, _ := run("-V")
	if ver == "" {
		ver = "unknown"
	}
	return fmt.Sprintf("(tmux %s, TERM=%s)", ver, os.Getenv("TERM"))
}

// wrapTopbarCmd wraps the topbar command so it auto-restarts on unexpected exit.
// A normal exit (status 0, from bay quitting) breaks the loop.
// Exits after 5 consecutive rapid failures to avoid spinning forever
// (e.g., if the binary is missing or consistently crashing).
func wrapTopbarCmd(cmd string) string {
	// Returns just the script body — callers must pass it to bash -c as a separate arg.
	// Split command to quote the binary path (handles spaces) while leaving args unquoted.
	parts := strings.SplitN(cmd, " ", 2)
	quoted := "'" + strings.ReplaceAll(parts[0], "'", "'\\''") + "'"
	if len(parts) > 1 {
		quoted += " " + parts[1]
	}
	return fmt.Sprintf(
		"fails=0; "+
			"while true; do "+
			"start=$SECONDS; "+
			"%s; "+
			"[ $? -eq 0 ] && break; "+
			"elapsed=$((SECONDS - start)); "+
			"if [ $elapsed -lt 2 ]; then "+
			"fails=$((fails + 1)); "+
			"else "+
			"fails=0; "+
			"fi; "+
			fmt.Sprintf("if [ $fails -ge %d ]; then ", constants.TopbarRestartMaxRetries)+
			fmt.Sprintf("echo \"bay topbar crashed %d times in a row, giving up\"; ", constants.TopbarRestartMaxRetries)+
			"break; "+
			"fi; "+
			"sleep "+constants.TopbarRestartDelay+"; "+
			"done", quoted)
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
		out, err := run("new-window", "-t", MainSession+":", "-d", "-P", "-F", "#{pane_id}", "bash", "-c", wrapped)
		if err != nil {
			return fmt.Errorf("new-window for topbar: %s: %w", out, err)
		}
		topbarPaneID = strings.TrimSpace(out)
		saveTopbarPaneID(topbarPaneID)
		return nil
	}

	if out, err := run("new-session", "-d", "-s", MainSession, "bash", "-c", wrapped); err != nil {
		// Collect diagnostics to help debug tmux failures on user machines.
		diag := collectDiagnostics()
		return fmt.Errorf("new-session: %s: %w\n%s", out, err, diag)
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
// Falls back to the persisted file so subprocesses (e.g. bay internal ensure-pane)
// can identify the topbar without having called InitOrAttach.
func TopbarPaneTarget() string {
	if topbarPaneID != "" {
		return topbarPaneID
	}
	if id := loadTopbarPaneID(); id != "" {
		topbarPaneID = id
		return id
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
	// -f makes it span the full window width, not just the target pane's column.
	if out, err := run("move-pane", "-fvb", "-s", target, "-t", targetPane, "-l", TopbarHeight); err != nil {
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
func BindKeysWithRunner(runner RunnerFunc, agentCmd string) error {
	return bindKeysImpl(runner, agentCmd)
}

// BindKeys sets up tmux options for the bay session.
func BindKeys(agentCmd string) error {
	return bindKeysImpl(run, agentCmd)
}

func bindKeysImpl(run RunnerFunc, agentCmd string) error {
	if agentCmd == "" {
		agentCmd = "claude"
	}
	// Mouse
	run("set-option", "-g", "mouse", "on")
	run("set-option", "-g", "focus-events", "on")
	// Prevent clicking window names in the status bar from switching windows.
	run("unbind-key", "-n", "MouseDown1Status")
	run("unbind-key", "-n", "MouseDown1StatusLeft")
	run("unbind-key", "-n", "MouseDown1StatusRight")

	// Pane border status
	run("set-option", "-g", "pane-border-status", "top")
	run("set-option", "-g", "pane-border-format", " #{?#{==:#{pane_index},0},bay,#{?#{pane_title},#{pane_title},#(basename #{pane_current_path})}} ")
	run("set-option", "-g", "pane-active-border-style", "fg=#F9FAFB,bold")
	run("set-option", "-g", "pane-border-style", "fg=#4B5563")

	// Status bar styling
	run("set-option", "-g", "status-style", "bg=#1F2937,fg=#9CA3AF")
	run("set-option", "-g", "status-left", " #{?client_prefix,⌘ CMD, bay} ")
	run("set-option", "-g", "status-left-style", "#{?client_prefix,bg=#7C3AED fg=#F9FAFB bold,bg=#374151 fg=#06B6D4 bold}")
	run("set-option", "-g", "status-right", "#(cat ~/.bay/.topbar-hints)")
	run("set-option", "-g", "status-right-length", "120")
	run("set-option", "-g", "status-interval", "1")
	run("set-option", "-g", "window-status-current-style", "fg=#06B6D4,bold")
	run("set-option", "-g", "window-status-style", "fg=#6B7280")
	// Hide window list from status bar — the topbar IS the session switcher.
	run("set-option", "-g", "window-status-format", "")
	run("set-option", "-g", "window-status-current-format", "")

	// Shell snippet that resolves the topbar pane ID at runtime from the persisted file.
	// This stays correct even after the topbar moves between windows.
	topbarIDFile := "$HOME/.bay/.topbar-pane-id"
	resizeTopbar := fmt.Sprintf("tmux resize-pane -t $(cat %s) -y %s 2>/dev/null || true", topbarIDFile, TopbarHeight)

	// Hooks: re-lock topbar height on layout-changing events.
	// after-split-window and after-kill-pane also sync pane layout to session YAML
	// so that pane state is persisted immediately, not just on session deactivation.
	syncPanes := "bay internal sync-panes &"
	for _, hook := range []string{"after-select-window", "client-session-changed"} {
		run("set-hook", "-g", hook,
			fmt.Sprintf("run-shell '%s'", resizeTopbar))
	}
	ensurePane := "bay internal ensure-pane &"
	run("set-hook", "-g", "after-kill-pane",
		fmt.Sprintf("run-shell '%s; %s'", resizeTopbar, ensurePane))
	// Clear title on new panes so they fall back to directory basename in the border.
	run("set-hook", "-g", "after-split-window", afterSplitHookCmd())

	// Backtick as a second prefix
	run("set-option", "-g", "prefix2", "`")
	run("bind-key", "`", "send-keys", "`")

	// Unbind native window-switching keys — all switching goes through topbar.
	run("unbind-key", "n") // next-window
	run("unbind-key", "p") // previous-window
	run("unbind-key", "l") // last-window
	for i := 1; i <= 9; i++ {
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

	// d/D for splits — guard against topbar pane, same pattern as w.
	run("bind-key", "-r", "d", "if-shell",
		fmt.Sprintf("[ \"#{pane_id}\" != \"$(cat %s)\" ]", topbarIDFile),
		fmt.Sprintf("run-shell 'tmux split-window -h -c \"#{pane_current_path}\" && %s'", resizeTopbar))
	run("bind-key", "-r", "D", "if-shell",
		fmt.Sprintf("[ \"#{pane_id}\" != \"$(cat %s)\" ]", topbarIDFile),
		fmt.Sprintf("run-shell 'tmux split-window -v -c \"#{pane_current_path}\" && %s'", resizeTopbar))

	// a for agent split — guard against topbar pane, same pattern as w.
	// bay agent generates a UUID, saves it to session YAML, then execs the configured agent.
	// Auto-labels the pane with the agent command so the border shows it.
	run("bind-key", "-r", "a", "if-shell",
		fmt.Sprintf("[ \"#{pane_id}\" != \"$(cat %s)\" ]", topbarIDFile),
		fmt.Sprintf("run-shell 'tmux split-window -h -c \"#{pane_current_path}\" '\"'\"'bash -c \"bay agent\"'\"'\"' && tmux select-pane -T %s && %s'", agentCmd, resizeTopbar))

	// w to close pane — guard against topbar AND last dev pane.
	// Count panes in the window, subtract 1 for topbar — only allow kill if >1 dev panes remain.
	run("bind-key", "-r", "w", "if-shell",
		fmt.Sprintf("[ \"#{pane_id}\" != \"$(cat %s)\" ] && [ $(tmux list-panes | wc -l) -gt 2 ]", topbarIDFile),
		fmt.Sprintf("run-shell 'tmux kill-pane && %s'", resizeTopbar))

	// Quick-access keybinds: send keys to topbar pane (pane 0 of current window)
	// Use .0 to dynamically target pane index 0 in the active window.
	// `+Space → focus topbar and toggle focused mode
	run("bind-key", "Space", "run-shell",
		"tmux send-keys -t .0 Space; tmux select-pane -t .0")

	// `+Tab → cycle session (repeatable: `+Tab+Tab+Tab...)
	run("bind-key", "-r", "Tab", "send-keys", "-t", ".0", "Tab")

	// `+r → cycle repo (repeatable)
	run("bind-key", "-r", "r", "send-keys", "-t", ".0", "r")

	// `+1-9 → jump to session by index (1-indexed, max 9 sessions)
	for i := 1; i <= 9; i++ {
		key := fmt.Sprintf("%d", i)
		run("bind-key", key, "send-keys", "-t", ".0", key)
	}

	// { and } to swap pane positions (up/down in tmux layout order)
	run("bind-key", "{", "run-shell",
		fmt.Sprintf("tmux swap-pane -U; %s; %s", resizeTopbar, syncPanes))
	run("bind-key", "}", "run-shell",
		fmt.Sprintf("tmux swap-pane -D; %s; %s", resizeTopbar, syncPanes))

	// , to rename pane — prompts for a label, sets it as the pane title.
	// Empty input clears the title (reverts to directory basename).
	run("bind-key", ",", "command-prompt", "-p", "pane name:", "select-pane -T '%%'")

	// Memory hooks: capture pane buffer on pane exit for episodic recording.
	// Also ensure a dev pane exists so the session doesn't get stuck with only the topbar.
	run("set-hook", "-g", "pane-exited",
		"run-shell 'bay internal capture #{pane_id} &; bay internal ensure-pane &'")

	return nil
}

// afterSplitHookCmd returns the run-shell command for the after-split-window hook.
func afterSplitHookCmd() string {
	topbarIDFile := "$HOME/.bay/.topbar-pane-id"
	resizeTopbar := fmt.Sprintf("tmux resize-pane -t $(cat %s) -y %s 2>/dev/null || true", topbarIDFile, TopbarHeight)
	return fmt.Sprintf("run-shell '%s; %s; %s'",
		"tmux select-pane -T ''", resizeTopbar, "bay internal sync-panes &")
}

// restoreAfterSplitHook re-sets the after-split-window hook after it was
// temporarily removed (e.g., during pane recreation).
func restoreAfterSplitHook() {
	run("set-hook", "-g", "after-split-window", afterSplitHookCmd())
}

// ListWindowIndices returns all window indices in the bay session.
func ListWindowIndices() []int {
	out, err := run("list-windows", "-t", MainSession, "-F", "#{window_index}")
	if err != nil {
		return nil
	}
	var indices []int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		indices = append(indices, idx)
	}
	return indices
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

// CaptureAllDevPanes captures all non-topbar, non-agent pane buffers in a window.
// Agent panes (running claude) are skipped since they produce garbage captures.
func CaptureAllDevPanes(windowIndex, lines int) (map[string]string, error) {
	// List all panes with their start command to filter agents
	sep := "%%BAYCAP%%"
	out, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex),
		"-F", fmt.Sprintf("#{pane_id}%s#{pane_start_command}", sep))
	if err != nil {
		return nil, fmt.Errorf("list-panes: %w", err)
	}

	topbarID := TopbarPaneTarget()
	result := make(map[string]string)

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, sep, 2)
		paneID := parts[0]
		startCmd := ""
		if len(parts) > 1 {
			startCmd = parts[1]
		}

		// Skip topbar and agent panes
		if paneID == topbarID {
			continue
		}
		if strings.Contains(startCmd, "bay agent") || strings.Contains(startCmd, "claude") {
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
	PaneID    string
	Command   string
	Cwd       string
	IsAgent   bool
	Title     string
	Activity  int64  // unix timestamp of last pane output (empty on tmux 3.6+)
	CursorPos string // "Y,X" cursor position for activity detection fallback
}

// SnapshotAllPanes queries all panes across all windows in the bay session.
// Returns a map of windowIndex → []PaneInfo (excluding the topbar pane).
func SnapshotAllPanes() map[int][]PaneInfo {
	sep := "%%BAY%%"
	out, err := run("list-panes", "-s", "-t", MainSession,
		"-F", fmt.Sprintf("#{window_index}%s#{pane_id}%s#{pane_start_command}%s#{pane_activity}%s#{cursor_y},#{cursor_x}", sep, sep, sep, sep))
	if err != nil {
		return nil
	}

	topbarID := TopbarPaneTarget()
	result := make(map[int][]PaneInfo)

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, sep, 5)
		if len(parts) < 5 {
			continue
		}

		winIdx, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		paneID := parts[1]
		startCmd := parts[2]
		activityStr := parts[3]
		cursorPos := parts[4]

		if paneID == topbarID {
			continue
		}

		isAgent := strings.Contains(startCmd, "bay agent") || strings.Contains(startCmd, "claude")
		activity, _ := strconv.ParseInt(activityStr, 10, 64)

		result[winIdx] = append(result[winIdx], PaneInfo{
			PaneID:    paneID,
			Command:   startCmd,
			IsAgent:   isAgent,
			Activity:  activity,
			CursorPos: cursorPos,
		})
	}

	return result
}

// SnapshotPaneLayout queries tmux for the current pane layout of a window.
// Returns all panes except the topbar pane.
func SnapshotPaneLayout(windowIndex int) ([]PaneInfo, error) {
	sep := "%%BAY%%"
	out, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex),
		"-F", fmt.Sprintf("#{pane_id}%s#{pane_title}%s#{pane_start_command}%s#{pane_current_path}", sep, sep, sep))
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

		parts := strings.SplitN(line, sep, 4)
		if len(parts) < 4 {
			continue
		}

		paneID := parts[0]
		title := parts[1]
		startCmd := parts[2]
		cwd := parts[3]

		// Skip topbar pane
		if paneID == topbarID {
			continue
		}

		// Detect agent panes by checking if the start command contains "bay agent" or "claude"
		isAgent := strings.Contains(startCmd, "bay agent") || strings.Contains(startCmd, "claude")
		panes = append(panes, PaneInfo{
			PaneID:  paneID,
			Command: startCmd,
			Cwd:     cwd,
			IsAgent: isAgent,
			Title:   title,
		})
	}

	return panes, nil
}

// RecreateSessionPanes recreates panes in a window from saved layout.
// The first pane already exists (created by CreateSessionWindow). If it should
// be an agent, we send the claude command to it. Additional panes are split.
func RecreateSessionPanes(windowIndex int, panes []SessionPane) error {
	// Suppress after-split-window hook during recreation to prevent
	// sync-panes from writing partial pane layouts to the session YAML.
	// Restored after all panes are created; the caller does a final sync.
	run("set-hook", "-gu", "after-split-window")
	defer restoreAfterSplitHook()

	// Find the first non-topbar pane — the topbar is moved into the window
	// before recreation, so pane 0 may be the topbar rather than the shell.
	topbarID := TopbarPaneTarget()
	firstPane := ""
	sep := "%%BAY%%"
	out, err := run("list-panes", "-t", fmt.Sprintf("%s:%d", MainSession, windowIndex),
		"-F", fmt.Sprintf("#{pane_id}%s#{pane_index}", sep))
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			parts := strings.SplitN(strings.TrimSpace(line), sep, 2)
			if len(parts) == 2 && parts[0] != topbarID {
				firstPane = fmt.Sprintf("%s:%d.%s", MainSession, windowIndex, parts[1])
				break
			}
		}
	}
	if firstPane == "" {
		// No non-topbar pane found — nothing to recreate into.
		return nil
	}

	// Select the dev pane so split-window targets it, not the topbar.
	run("select-pane", "-t", firstPane)

	for i, p := range panes {
		if i == 0 {
			// First pane already exists — launch agent if needed, set title
			if p.Type == "agent" {
				agentCmd := agentLaunchCmd(p.AgentSessionID)
				// Use respawn-pane to replace the shell directly instead of
				// send-keys. send-keys races with shell initialization and
				// can cause claude to crash in the half-ready environment.
				run("respawn-pane", "-k", "-t", firstPane, agentCmd)
			}
			if p.Title != "" {
				run("select-pane", "-t", firstPane, "-T", p.Title)
			} else {
				run("select-pane", "-t", firstPane, "-T", "")
			}
			continue
		}

		dir := p.Cwd
		if dir == "" {
			continue
		}

		args := []string{"split-window", "-h", "-t", firstPane, "-c", dir}
		if p.Type == "agent" {
			// split-window runs the command directly as the pane process —
			// no bash -c wrapper needed.
			args = append(args, agentLaunchCmd(p.AgentSessionID))
		}
		if _, err := run(args...); err != nil {
			continue // Non-fatal: best-effort recreation
		}

		if p.Title != "" {
			run("select-pane", "-T", p.Title)
		} else {
			run("select-pane", "-T", "")
		}
	}

	// Lock topbar height after adding panes
	run("resize-pane", "-t", TopbarPaneTarget(), "-y", TopbarHeight)

	return nil
}

// SessionPane is a minimal pane description used for recreation.
// This mirrors session.Pane but avoids the import cycle.
type SessionPane struct {
	Type            string
	Cwd             string
	Command         string
	Title           string
	AgentSessionID string
}

// agentLaunchCmd returns the shell command to launch an agent pane.
// If a Claude session ID exists, resumes it; otherwise starts fresh.
func agentLaunchCmd(agentSessionID string) string {
	if agentSessionID != "" {
		return fmt.Sprintf("bay agent --resume %s", agentSessionID)
	}
	return "bay agent"
}

// hintsFile returns the path where topbar hints are written for tmux status bar.
func hintsFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bay", ".topbar-hints")
}

// WriteTopbarHints writes plain-text hints to disk for tmux status-right to display.
// Forces an immediate status bar refresh so the change is visible without waiting
// for the next status-interval tick.
func WriteTopbarHints(hints string) {
	f := hintsFile()
	os.MkdirAll(filepath.Dir(f), 0o755)
	os.WriteFile(f, []byte(hints), 0o644)
	run("refresh-client", "-S")
}

// EnsureDevPane checks if any non-topbar panes exist in the given window.
// If not, it spawns a new terminal pane so the session doesn't get stuck.
func EnsureDevPane(windowIndex int, dir string) error {
	panes, err := SnapshotPaneLayout(windowIndex)
	if err != nil {
		return nil // Window may already be gone
	}
	if len(panes) > 0 {
		return nil // Dev panes still exist — nothing to do
	}

	// No dev panes remain — split a plain shell so the session isn't stuck.
	// Spawning an agent here would cause a death spiral if the agent crashes
	// immediately (pane-exited → ensure-pane → agent → crash → repeat).
	target := fmt.Sprintf("%s:%d", MainSession, windowIndex)
	args := []string{"split-window", "-v", "-t", target}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if _, err := run(args...); err != nil {
		return fmt.Errorf("ensure-dev-pane split-window: %w", err)
	}

	// Lock topbar height back to 5 lines.
	run("resize-pane", "-t", TopbarPaneTarget(), "-y", TopbarHeight)
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
