package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bay/internal/config"
)

var (
	logger *log.Logger
	file   *os.File
	once   sync.Once
)

// Init sets up file-based logging to ~/.bay/logs/bay.log.
// Safe to call multiple times — only the first call takes effect.
func Init() {
	once.Do(func() {
		dir := filepath.Join(config.BayDir(), "logs")
		os.MkdirAll(dir, 0755)

		logPath := filepath.Join(dir, "bay.log")

		var err error
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fall back to stderr if we can't open the log file
			logger = log.New(os.Stderr, "[bay] ", log.Ldate|log.Ltime|log.Lshortfile)
			return
		}

		logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)

		// Rotate if over 5MB
		if info, err := file.Stat(); err == nil && info.Size() > 5*1024*1024 {
			rotate(logPath)
		}
	})
}

// rotate renames the current log file and opens a fresh one.
func rotate(logPath string) {
	if file != nil {
		file.Close()
	}
	ts := time.Now().Format("20060102-150405")
	rotated := logPath + "." + ts
	os.Rename(logPath, rotated)

	var err error
	file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger = log.New(os.Stderr, "[bay] ", log.Ldate|log.Ltime|log.Lshortfile)
		return
	}
	logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)

	// Clean up old rotated logs (keep last 3)
	cleanOldLogs(filepath.Dir(logPath))
}

// cleanOldLogs removes rotated log files beyond the 3 most recent.
func cleanOldLogs(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "bay.log.*"))
	if len(matches) <= 3 {
		return
	}
	// Glob returns sorted order, oldest first
	for _, f := range matches[:len(matches)-3] {
		os.Remove(f)
	}
}

func getLogger() *log.Logger {
	if logger == nil {
		Init()
	}
	return logger
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
	if file != nil {
		file.Close()
	}
}
