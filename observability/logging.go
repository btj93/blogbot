package observability

import (
	"log/slog"
	"os"
	"strings"
)

func InitLogging(level string) {
	var l slog.Level

	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	h := NewContextHandler(
		slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: l}),
		[]ContextKey{RequestIDKey{}, RunIDKey{}, CommandKey{}},
	)
	slog.SetDefault(slog.New(h))
}
