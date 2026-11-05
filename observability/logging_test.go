package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactTelegramTokens(t *testing.T) {
	const token = "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ_123456789"
	for _, input := range []string{
		token,
		"Post https://api.telegram.org/bot" + token + "/sendMessage: timeout",
		strings.ReplaceAll(token, ":", "%3A"),
	} {
		got := RedactTelegramTokens(input)
		if strings.Contains(got, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") || !strings.Contains(got, "[REDACTED]") {
			t.Fatalf("credential not redacted: %q", got)
		}
	}

	const normal = "chat_id=-10012345 member_id=12 time=12:30"
	if got := RedactTelegramTokens(normal); got != normal {
		t.Fatalf("redacted diagnostic data: %q", got)
	}
}

func TestDiagnosticContextAndLogRedaction(t *testing.T) {
	var output bytes.Buffer

	handler := NewContextHandler(slog.NewJSONHandler(&output, &slog.HandlerOptions{ReplaceAttr: redactLogAttr}),
		[]ContextKey{RunIDKey{}, CommandKey{}})
	logger := slog.New(handler)
	base := WithAttrs(WithRunID(context.Background(), "scrape"), slog.String("group", "test"))
	ctx := WithAttrs(base, slog.Int64("member_id", 12), slog.String("blog_url", "https://example.com/post/1"))

	const token = "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ_123456789"
	logger.ErrorContext(
		ctx,
		"test event",
		slog.Group("details", slog.Any("error", errors.New("bot"+token+"/sendMessage"))),
		slog.String("url", "https://api.telegram.org/bot"+token+"/getWebhookInfo"),
	)

	if strings.Contains(output.String(), token) {
		t.Fatal("JSON logs exposed bot token")
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"run_id", "command", "group", "member_id", "blog_url"} {
		if record[key] == nil {
			t.Errorf("missing context field %s: %s", key, output.String())
		}
	}

	output.Reset()
	logger.InfoContext(base, "parent context")

	if strings.Contains(output.String(), "member_id") {
		t.Fatal("child fields leaked into parent context")
	}
}
