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

var logFile    *os.File
var logFileDate string // YYYY-MM-DD of the currently open log file

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
	if err := openLogFileForDate(time.Now().Format("2006-01-02")); err != nil {
		return err
	}
	Info("LOGGING", fmt.Sprintf("Log file opened: %s", logFile.Name()))
	return nil
}

// openLogFileForDate opens (or creates) the log file for the given date,
// closing any previously open file first.
func openLogFileForDate(date string) error {
	dir, err := getLogDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("bridge-ground-%s.log", date))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", path, err)
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile     = f
	logFileDate = date
	mw := io.MultiWriter(os.Stderr, f)
	log.SetOutput(mw)
	log.SetFlags(0)
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
// Automatically rotates to a new file when the calendar date changes.
func Write(level, tag, message string) {
	now := time.Now()
	today := now.Format("2006-01-02")
	if logFile != nil && logFileDate != today {
		// Date has changed — rotate to a new log file silently.
		_ = openLogFileForDate(today)
	}
	timestamp := now.Format("2006-01-02 15:04:05.000")
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
	today := time.Now().Format("2006-01-02")
	return GetEntriesForRange(today, today)
}

// ReadAppLogsFromDir reads log files from an external app's log directory.
// Files must follow the naming pattern: {prefix}-YYYY-MM-DD.log
// and use the same line format as Bridge-Ground's own logs.
func ReadAppLogsFromDir(dir, prefix, from, to string) []LogEntry {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	expectedPrefix := prefix + "-"
	var result []LogEntry
	for _, de := range dirEntries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".log" {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, expectedPrefix) {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, expectedPrefix), ".log")
		if dateStr < from || dateStr > to {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			if m := logLineRe.FindStringSubmatch(line); m != nil {
				result = append(result, LogEntry{Timestamp: m[1], Level: m[2], Tag: m[3], Message: m[4]})
			} else {
				result = append(result, LogEntry{Message: line})
			}
		}
	}
	return result
}

// GetEntriesForRange reads all log files whose date falls in [from, to] (YYYY-MM-DD)
// and returns the combined entries in chronological order.
func GetEntriesForRange(from, to string) []LogEntry {
	dir, err := getLogDir()
	if err != nil {
		return nil
	}
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []LogEntry
	for _, de := range dirEntries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".log" {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, "bridge-ground-") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "bridge-ground-"), ".log")
		if dateStr < from || dateStr > to {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			if m := logLineRe.FindStringSubmatch(line); m != nil {
				result = append(result, LogEntry{Timestamp: m[1], Level: m[2], Tag: m[3], Message: m[4]})
			} else {
				result = append(result, LogEntry{Message: line})
			}
		}
	}
	return result
}
