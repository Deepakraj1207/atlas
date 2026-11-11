// Package logging sets up structured logging to a file. The TUI owns the
// terminal, so nothing may ever be logged to stdout or stderr while it runs.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/deepakraj1207/atlas/internal/config"
)

// FileName is the log file name inside the log directory.
const FileName = "atlas.log"

// Setup opens (appending) the log file in dir and returns a JSON-lines
// logger. The returned close function must be called on exit.
func Setup(dir string, level config.LogLevel) (*slog.Logger, func(), error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log folder: %w", err)
	}
	path := filepath.Join(dir, FileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	logger := New(f, level)
	return logger, func() { _ = f.Close() }, nil
}

// New builds a JSON logger writing to w at the given level.
func New(w io.Writer, level config.LogLevel) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: toSlog(level)})
	return slog.New(h)
}

// Discard returns a logger that drops everything. Used when the log file
// cannot be opened so the app still starts.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func toSlog(l config.LogLevel) slog.Level {
	switch l {
	case config.LogDebug:
		return slog.LevelDebug
	case config.LogWarn:
		return slog.LevelWarn
	case config.LogError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
