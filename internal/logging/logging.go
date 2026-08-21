// Package logging provides structured, leveled logging for the benchmark.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// New returns a structured JSON logger writing to w at the given level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
}

// Default returns a logger writing to stderr at Info level.
func Default() *slog.Logger {
	return New(os.Stderr, slog.LevelInfo)
}
