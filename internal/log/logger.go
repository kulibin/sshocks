package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level is a log severity.
type Level int

const (
	// DebugLevel is the most verbose level.
	DebugLevel Level = iota
	// InfoLevel is the default level.
	InfoLevel
	// WarnLevel reports recoverable problems.
	WarnLevel
	// ErrorLevel reports failures.
	ErrorLevel
)

// ParseLevel converts a config string into a Level. Empty/unknown falls back to
// InfoLevel.
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return DebugLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "", "info":
		return InfoLevel
	default:
		return InfoLevel
	}
}

// LevelName returns the uppercase name used in log lines.
func (l Level) LevelName() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Logger is the logging interface used across the application.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Flush()
	Close() error
}

// fileLogger writes timestamped lines to a file.
type fileLogger struct {
	mu     sync.Mutex
	out    *os.File
	level  Level
	closed bool
}

// NewFile opens logFile for append and returns a level-aware file logger. The
// parent directory is created if needed.
func NewFile(path, level string) (*fileLogger, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &fileLogger{out: f, level: ParseLevel(level)}, nil
}

func (l *fileLogger) write(level Level, format string, args ...any) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.out == nil {
		return
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf(format, args...)
	_, err := fmt.Fprintf(l.out, "%s %s %s\n", ts, level.LevelName(), line)
	_ = err
}

// Debugf writes at DebugLevel.
func (l *fileLogger) Debugf(format string, args ...any) { l.write(DebugLevel, format, args...) }

// Infof writes at InfoLevel.
func (l *fileLogger) Infof(format string, args ...any) { l.write(InfoLevel, format, args...) }

// Warnf writes at WarnLevel.
func (l *fileLogger) Warnf(format string, args ...any) { l.write(WarnLevel, format, args...) }

// Errorf writes at ErrorLevel.
func (l *fileLogger) Errorf(format string, args ...any) { l.write(ErrorLevel, format, args...) }

// Flush flushes any buffered output.
func (l *fileLogger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.out == nil {
		return
	}
	_ = l.out.Sync()
}

// Close flushes and closes the underlying file.
func (l *fileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.out == nil {
		return nil
	}
	err := l.out.Close()
	l.out = nil
	return err
}

// discardLogger is a no-op logger used in tests and when logging is disabled.
type discardLogger struct{}

// Discard returns a no-op Logger.
func Discard() Logger {
	return discardLogger{}
}

func (discardLogger) Debugf(format string, args ...any) {}
func (discardLogger) Infof(format string, args ...any)  {}
func (discardLogger) Warnf(format string, args ...any)  {}
func (discardLogger) Errorf(format string, args ...any) {}
func (discardLogger) Flush()                            {}
func (discardLogger) Close() error                      { return nil }
