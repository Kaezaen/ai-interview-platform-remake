// Package logging builds the application's structured logger and carries a
// request-scoped logger through context.Context.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// New returns a logger that writes text or JSON records at the given level.
func New(w io.Writer, format string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

// ctxKey is unexported so no other package can read or overwrite the value.
type ctxKey struct{}

// WithLogger returns a copy of ctx that carries l.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the logger stored by WithLogger. Outside a request
// (or in tests that skip the middleware) it falls back to slog.Default, so
// callers never have to nil-check.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
