package observability

import (
	"log/slog"
	"os"
	"regexp"
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
		slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: l, ReplaceAttr: redactLogAttr}),
		[]ContextKey{RequestIDKey{}, RunIDKey{}, CommandKey{}},
	)
	slog.SetDefault(slog.New(h))
}

var telegramTokenPattern = regexp.MustCompile(`[0-9]+(?::|%3[Aa])[A-Za-z0-9_-]{20,}`)

// RedactTelegramTokens removes bot credentials, including those inside HTTP errors.
func RedactTelegramTokens(text string) string {
	return telegramTokenPattern.ReplaceAllString(text, "[REDACTED]")
}

func redactLogAttr(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(RedactTelegramTokens(attr.Value.String()))
	case slog.KindAny:
		if err, ok := attr.Value.Any().(error); ok {
			attr.Value = slog.StringValue(RedactTelegramTokens(err.Error()))
		}
	default:
	}

	return attr
}
