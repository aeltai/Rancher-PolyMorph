package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Logger writes human-readable progress and diagnostics to stderr (and optionally a log file).
type Logger struct {
	mu      sync.Mutex
	quiet   bool
	verbose bool
	err     io.Writer
	log     io.Writer
	start   time.Time
}

func NewLogger(quiet, verbose bool, logPath string) (*Logger, error) {
	l := &Logger{
		quiet:   quiet,
		verbose: verbose,
		err:     os.Stderr,
		start:   time.Now(),
	}
	if logPath != "" {
		f, err := os.Create(logPath)
		if err != nil {
			return nil, fmt.Errorf("create log file: %w", err)
		}
		l.log = f
		l.Info("log file: %s", logPath)
	}
	return l, nil
}

func (l *Logger) Close() error {
	if closer, ok := l.log.(io.Closer); ok && closer != nil {
		return closer.Close()
	}
	return nil
}

func (l *Logger) write(level, format string, args ...any) {
	if l.quiet && level != "ERROR" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %-5s %s\n", ts, level, msg)
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.err.Write([]byte(line))
	if l.log != nil {
		_, _ = l.log.Write([]byte(line))
	}
}

func (l *Logger) Info(format string, args ...any)  { l.write("INFO", format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.write("WARN", format, args...) }
func (l *Logger) Error(format string, args ...any) { l.write("ERROR", format, args...) }
func (l *Logger) OK(format string, args ...any)    { l.write("OK", format, args...) }
func (l *Logger) Step(format string, args ...any)  { l.write("STEP", format, args...) }

func (l *Logger) Debug(format string, args ...any) {
	if l.verbose {
		l.write("DEBUG", format, args...)
	}
}

func (l *Logger) Elapsed() time.Duration {
	return time.Since(l.start)
}

// LogWriter returns the optional log file writer (nil if no --log-file).
func (l *Logger) LogWriter() io.Writer {
	return l.log
}
