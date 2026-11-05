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

	attrs, _ := ctx.Value(logAttrsKey{}).([]slog.Attr)
	as = append(as, attrs...)

	return as
}

type logAttrsKey struct{}

// WithAttrs attaches trusted diagnostic fields to logs made with this context.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	previous, _ := ctx.Value(logAttrsKey{}).([]slog.Attr)
	combined := append(append([]slog.Attr(nil), previous...), attrs...)

	return context.WithValue(ctx, logAttrsKey{}, combined)
}

func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs), keys: h.keys}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name), keys: h.keys}
}
