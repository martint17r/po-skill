package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// newLogHandler returns tint on a terminal, JSON otherwise (T13, ADR 0003).
func newLogHandler(w io.Writer, isTerminal bool) slog.Handler {
	if isTerminal {
		return tint.NewHandler(w, nil)
	}
	return slog.NewJSONHandler(w, nil)
}

// setupLogging installs the default logger; f counts as a terminal if it is a character device.
func setupLogging(f *os.File) {
	fi, err := f.Stat()
	slog.SetDefault(slog.New(newLogHandler(f, err == nil && fi.Mode()&os.ModeCharDevice != 0)))
}
