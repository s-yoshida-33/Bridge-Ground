package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LogEntry is a structured log line.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
}

var logLineRe = regexp.MustCompile(`^\[([^\]]+)\] \[([^\]]+)\] \[([^\]]+)\] (.*)$`)

var logFile *os.File

// getLogDir returns (and creates) %APPDATA%\TTI\BridgeGround\logs.
func getLogDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "TTI", "BridgeGround", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log dir: %w", err)
	}
	return dir, nil
}

// getLogFilePath returns today's log file path.
func getLogFilePath() (string, error) {
	dir, err := getLogDir()
	if err != nil {
		return "", err
	}
	today := time.Now().Format("2006-01-02")
	return filepath.Join(dir, fmt.Sprintf("bridge-ground-%s.log", today)), nil
}

// Init opens today's log file and redirects Go's standard logger to write to
// both stderr and the file. Call once at program start.
func Init() error {
	path, err := getLogFilePath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", path, err)
	}
	logFile = f

	// Redirect Go's standard logger to stderr + file, with no automatic prefix
	// (we add our own timestamp/level/tag prefix in Write).
	mw := io.MultiWriter(os.Stderr, f)
	log.SetOutput(mw)
	log.SetFlags(0)

	Info("LOGGING", fmt.Sprintf("Log file opened: %s", path))
	return nil
}

// CleanupOldLogs removes .log files in the log directory older than maxAgeDays.
func CleanupOldLogs(maxAgeDays int) {
	dir, err := getLogDir()
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

// Close flushes and closes the log file. Call via defer in main.
func Close() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// Write writes a structured log entry to the file (and stderr).
// Format: [YYYY-MM-DD HH:MM:SS.mmm] [LEVEL] [TAG] message
func Write(level, tag, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	entry := fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level, tag, message)
	fmt.Fprint(os.Stderr, entry)
	if logFile != nil {
		_, _ = logFile.WriteString(entry)
	}
}

func Info(tag, message string)  { Write("INFO", tag, message) }
func Warn(tag, message string)  { Write("WARN", tag, message) }
func Error(tag, message string) { Write("ERROR", tag, message) }
func Fatal(tag, message string) { Write("FATAL", tag, message) }

// GetRecentEntries reads the current day's log file and returns the last n entries.
func GetRecentEntries(n int) []LogEntry {
	path, err := getLogFilePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	entries := make([]LogEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		m := logLineRe.FindStringSubmatch(line)
		if m != nil {
			entries = append(entries, LogEntry{Timestamp: m[1], Level: m[2], Tag: m[3], Message: m[4]})
		} else {
			entries = append(entries, LogEntry{Message: line})
		}
	}
	return entries
}
