package memory

// --- Buffer classification ---

// MeaningfulLineThreshold is the minimum number of non-prompt, non-blank lines
// required for a pane buffer to be considered "non-trivial" and worth summarizing.
const MeaningfulLineThreshold = 3

// --- Pending summary retry policy ---

const (
	// MaxRetries is the maximum number of LLM summarization attempts per buffer
	// before the pending entry is considered permanently failed.
	MaxRetries = 3

	// PendingMaxAge is the SQLite datetime modifier for the oldest pending
	// summary that is still eligible for retry (e.g. "-1 hour").
	PendingMaxAge = "-1 hour"
)

// --- Summary compaction ---

const (
	// CompactThreshold is the number of summary entries per session above which
	// compaction is triggered (oldest N summaries are condensed into one via LLM).
	CompactThreshold = 10

	// MinCompactEntries is the minimum number of summaries required to perform
	// a compaction pass — with fewer entries, compaction is a no-op.
	MinCompactEntries = 2
)
