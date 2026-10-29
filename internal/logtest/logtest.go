// Package logtest captures what code logs through the default slog logger.
package logtest

import (
	"bytes"
	"log"
	"log/slog"
	"testing"
)

// Capture sends the default logger's records to the returned buffer, as
// JSON at debug level and up, until the test ends. It swaps a global, so
// tests that call it must not run in parallel.
func Capture(t testing.TB) *bytes.Buffer {
	t.Helper()
	// slog.SetDefault also points the log package at the new handler, and
	// setting the old default back does not undo that.
	prev, out, flags := slog.Default(), log.Writer(), log.Flags()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		log.SetOutput(out)
		log.SetFlags(flags)
	})
	return &buf
}
