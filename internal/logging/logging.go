// Package logging provides file-based logging to ~/.bay/logs/bay.log.
//
// Init() is safe to call multiple times — sync.Once ensures only the first
// invocation opens the log file. If the file cannot be opened, logging falls
// back to stderr so callers never need to handle initialization errors.
//
// Log rotation happens automatically when the file exceeds a configurable size
// threshold. Rotated files are renamed with a timestamp suffix and only the last
// N rotated files are kept, preventing unbounded disk growth.
//
// All public functions (Info, Warn, Error, Debug) and Close() are thread-safe:
// getLogger() and Close() are protected by a shared mutex so concurrent
// goroutines (topbar TUI, background summarizers) can log without races.
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bay/internal/config"
	"bay/internal/constants"
)

var (
	logger *log.Logger
	file   *os.File
	mu     sync.Mutex
	once   sync.Once
)

// Init sets up file-based logging to ~/.bay/logs/bay.log.
// Safe to call multiple times — only the first call takes effect.
func Init() {
	once.Do(func() {
		dir := filepath.Join(config.BayDir(), "logs")
		os.MkdirAll(dir, 0o755) // best-effort; fallback to stderr below

		logPath := filepath.Join(dir, "bay.log")

		// Rotate before opening if the file exceeds the size limit.
		if info, err := os.Stat(logPath); err == nil && info.Size() > constants.MaxLogFileSize {
			rotateFile(logPath)
		}

		var err error
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logger = log.New(os.Stderr, "[bay] ", log.Ldate|log.Ltime|log.Lshortfile)
			return
		}

		logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)
	})
}

// rotateFile renames the current log file with a timestamp suffix.
func rotateFile(logPath string) {
	ts := time.Now().Format(constants.LogTimeFmt)
	os.Rename(logPath, logPath+"."+ts)

	// Clean up old rotated logs (keep last 3).
	// filepath.Glob returns lexical order, which is chronological
	// because the timestamp format sorts lexically.
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(logPath), "bay.log.*"))
	if len(matches) > constants.MaxOldLogFiles {
		for _, f := range matches[:len(matches)-constants.MaxOldLogFiles] {
			os.Remove(f)
		}
	}
}

func getLogger() *log.Logger {
	once.Do(func() { Init() })
	mu.Lock()
	l := logger
	mu.Unlock()
	return l
}

// Info logs an informational message.
func Info(format string, args ...any) {
	getLogger().Output(2, fmt.Sprintf("INFO  "+format, args...))
}

// Error logs an error message.
func Error(format string, args ...any) {
	getLogger().Output(2, fmt.Sprintf("ERROR "+format, args...))
}

// Debug logs a debug message.
func Debug(format string, args ...any) {
	getLogger().Output(2, fmt.Sprintf("DEBUG "+format, args...))
}

// Warn logs a warning message.
func Warn(format string, args ...any) {
	getLogger().Output(2, fmt.Sprintf("WARN  "+format, args...))
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
}
