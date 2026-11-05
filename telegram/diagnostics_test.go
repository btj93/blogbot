package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/observability"
)

func captureTelegramLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(
		slog.New(
			observability.NewContextHandler(
				slog.NewJSONHandler(&logs, nil),
				[]observability.ContextKey{observability.RunIDKey{}},
			),
		),
	)
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &logs
}

func diagnosticClient(t *testing.T, status int) (*Client, <-chan string) {
	t.Helper()

	texts := make(chan string, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test","username":"test_bot"}}`)
		case status == http.StatusForbidden:
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`)
		case strings.HasSuffix(r.URL.Path, "/sendMediaGroup"):
			_, _ = io.WriteString(w, `{"ok":true,"result":[{"message_id":11},{"message_id":12}]}`)
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}

			texts <- r.FormValue("text")

			_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":10}}`)
		default:
			t.Errorf("unexpected Telegram method: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(
		"123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ_123456789",
		server.URL+"/bot%s/%s",
	)
	if err != nil {
		t.Fatal(err)
	}

	return &Client{bot: bot, logChatID: 999, maxRetries: 3}, texts
}

func TestTelegramAcceptedDiagnostics(t *testing.T) {
	logs := captureTelegramLogs(t)
	client, _ := diagnosticClient(t, http.StatusOK)

	ctx := observability.WithAttrs(observability.WithRunID(context.Background(), "scrape"),
		slog.Int64("member_id", 7), slog.String("blog_url", "https://example.com/post/1"))
	if err := client.SendText(ctx, 42, "notification", nil); err != nil {
		t.Fatal(err)
	}

	if err := client.SendMediaGroup(ctx, 42, [][]byte{[]byte("a"), []byte("b")}); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two accepted events, got %s", logs.String())
	}

	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}

		if record["msg"] != "telegram send accepted" || record["chat_id"] != float64(42) ||
			record["member_id"] != float64(7) ||
			record["blog_url"] == nil ||
			record["run_id"] == nil {
			t.Errorf("missing correlation: %s", line)
		}

		if record["message_id"] == nil && record["message_ids"] == nil {
			t.Errorf("missing Telegram message IDs: %s", line)
		}
	}
}

func TestTelegramTerminalFailureDiagnostics(t *testing.T) {
	logs := captureTelegramLogs(t)
	client, _ := diagnosticClient(t, http.StatusForbidden)
	before := observability.TelegramSendErrors.Value()

	if err := client.SendText(context.Background(), 42, "notification", nil); err == nil {
		t.Fatal("expected blocked-user error")
	}

	if got := observability.TelegramSendErrors.Value() - before; got != 1 {
		t.Fatalf("terminal error count=%d, want 1", got)
	}

	if strings.Contains(logs.String(), "telegram send accepted") ||
		!strings.Contains(logs.String(), `"telegram_error_code":403`) ||
		!strings.Contains(logs.String(), `"chat_id":42`) {
		t.Fatalf("incorrect failure event: %s", logs.String())
	}
}

func TestTelegramLogTextRedactsCredentials(t *testing.T) {
	client, texts := diagnosticClient(t, http.StatusOK)
	client.LogText(
		context.Background(),
		"request failed: https://api.telegram.org/bot123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ_123456789/sendMessage",
	)

	text := <-texts
	if strings.Contains(text, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") || !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("unsafe admin log: %q", text)
	}
}
