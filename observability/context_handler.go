package observability

import (
	"context"
	"log/slog"
)

// ContextKey is a typed key for extracting values from context into log attributes.
type ContextKey interface {
	Key() string
}

// ContextHandler wraps a slog.Handler and automatically injects context values as log attributes.
type ContextHandler struct {
	slog.Handler
	keys []ContextKey
}

// NewContextHandler creates a handler that extracts registered context keys on every log call.
func NewContextHandler(handler slog.Handler, keys []ContextKey) *ContextHandler {
	return &ContextHandler{
		Handler: handler,
		keys:    keys,
	}
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.Handler.WithAttrs(h.observe(ctx)).Handle(ctx, r)
}

func (h ContextHandler) observe(ctx context.Context) []slog.Attr {
	as := make([]slog.Attr, 0, len(h.keys))

	for _, k := range h.keys {
		v := ctx.Value(k)
		if v == nil {
			continue
		}

		as = append(as, slog.Any(k.Key(), v))
	}

	return as
}

func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs), keys: h.keys}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name), keys: h.keys}
}
