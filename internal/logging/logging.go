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
	mu     sync.Mutex
	once   sync.Once
)

// Init sets up file-based logging to ~/.bay/logs/bay.log.
// Safe to call multiple times — only the first call takes effect.
func Init() {
	once.Do(func() {
		dir := filepath.Join(config.BayDir(), "logs")
		os.MkdirAll(dir, 0755) // best-effort; fallback to stderr below

		logPath := filepath.Join(dir, "bay.log")

		// Rotate before opening if the file is over 5MB.
		if info, err := os.Stat(logPath); err == nil && info.Size() > 5*1024*1024 {
			rotateFile(logPath)
		}

		var err error
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger = log.New(os.Stderr, "[bay] ", log.Ldate|log.Ltime|log.Lshortfile)
			return
		}

		logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)
	})
}

// rotateFile renames the current log file with a timestamp suffix.
func rotateFile(logPath string) {
	ts := time.Now().Format("20060102-150405")
	os.Rename(logPath, logPath+"."+ts)

	// Clean up old rotated logs (keep last 3).
	// filepath.Glob returns lexical order, which is chronological
	// because the timestamp format sorts lexically.
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(logPath), "bay.log.*"))
	if len(matches) > 3 {
		for _, f := range matches[:len(matches)-3] {
			os.Remove(f)
		}
	}
}

func getLogger() *log.Logger {
	once.Do(func() { Init() })
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
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
}
