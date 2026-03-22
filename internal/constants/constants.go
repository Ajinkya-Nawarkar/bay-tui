// Package constants defines shared magic values used across the bay codebase.
// Centralizing these prevents silent divergence when values are duplicated
// and makes tuning easier (grep for the constant name, not a bare number).
package constants

import "time"

// --- CLI / Agent ---

// DefaultAgent is the agent command used when none is configured.
const DefaultAgent = "claude"

// --- Display ---

// ContentTruncateLen is the max display length before "..." is appended.
const ContentTruncateLen = 120

// DefaultHistoryLimit is how many episodic entries `bay session history` shows.
const DefaultHistoryLimit = 20

// --- Terminal ---

// TermFallback is the TERM value used when the current one lacks a terminfo entry.
const TermFallback = "xterm-256color"

// --- Time formats (Go reference-time based) ---

const (
	TimeFmtFull    = "2006-01-02 15:04:05"
	TimeFmtShort   = "15:04:05"
	TimeFmtCompact = "2006-01-02 15:04"
)

// --- Logging ---

const (
	// MaxLogFileSize triggers log rotation when exceeded (5 MB).
	MaxLogFileSize = 5 * 1024 * 1024

	// MaxOldLogFiles is the number of rotated logs to keep.
	MaxOldLogFiles = 3

	// LogTimeFmt is the timestamp suffix appended to rotated log files.
	LogTimeFmt = "20060102-150405"
)

// --- Tmux / Pane capture ---

// PaneCaptureBuffer is the number of terminal lines captured per pane snapshot.
const PaneCaptureBuffer = 100

// --- Topbar restart loop ---

const (
	TopbarRestartMaxRetries = 5
	TopbarRestartDelay      = "0.5" // seconds, used in shell sleep
)

// --- Agent activity detection ---

const (
	// AgentActivityThreshold is how recently (in seconds) a pane must have
	// had activity to be considered "active" vs "idle".
	AgentActivityThreshold = int64(3)

	// DiffCacheTTL is how long a cached git diff summary is valid.
	DiffCacheTTL = 10 * time.Second

	// AgentTickInterval is the polling interval for agent activity + diff refresh.
	AgentTickInterval = 2 * time.Second
)

// --- Session limits ---

// MaxSessionsPerRepo is the maximum number of sessions allowed per repository.
const MaxSessionsPerRepo = 9

// StaleDays is the number of days after which an inactive session is considered stale.
const StaleDays = 30

// --- TUI layout ---

const (
	MinTermWidth    = 20
	DefaultTermWidth = 80
	CleanupPageSize = 5
)

// --- Status message durations ---

const (
	StatusClearDuration = 2 * time.Second
	StatusClearLong     = 3 * time.Second
)

// --- Unicode glyphs used in the topbar ---

const (
	NavLeft  = "\u25c0" // ◀
	NavRight = "\u25b6" // ▶
)

// --- Session marker files ---

const (
	ActiveSessionFile  = ".active-session"
	CreatedSessionFile = ".created-session"
)
