// Package logging configures the process-wide slog logger. While the TUI owns the terminal
// nothing may be written to stdout/stderr, so logs go to a file and only when asked for.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// DefaultPath is the log file used with --debug when --log is not given.
func DefaultPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "incott", "incott.log")
}

// Setup directs slog to path at debug level. With debug=false logging is discarded.
// The returned closer must be closed on exit.
func Setup(path string, debug bool) (io.Closer, error) {
	if !debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return nopCloser{}, nil
	}
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return f, nil
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
