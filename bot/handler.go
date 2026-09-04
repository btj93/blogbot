package bot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

type Handler struct {
	tg        telegram.Sender
	db        store.Querier
	botToken  string
	webAppURL string
	useWebApp bool
}

func NewHandler(tg telegram.Sender, db store.Querier, botToken, webAppURL string, useWebApp bool) *Handler {
	return &Handler{tg: tg, db: db, botToken: botToken, webAppURL: webAppURL, useWebApp: useWebApp}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := observability.WithRequestID(r.Context())
	ctx = observability.WithCommand(ctx, "webhook")

	observability.WebhookRequests.Add(1)
	observability.PromWebhookRequests.Inc()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	h.processUpdate(ctx, &update)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) processUpdate(ctx context.Context, update *tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "recovered from panic in processUpdate", slog.Any("panic", r))
		}
	}()

	if update.CallbackQuery != nil {
		if h.useWebApp {
			if err := h.tg.AnswerCallbackQuery(ctx, update.CallbackQuery.ID, "請使用 /editsublist 開啟新版訂閱管理"); err != nil {
				slog.ErrorContext(ctx, "failed to answer stale callback", slog.Any("error", err))
			}

			return
		}

		handleCallback(ctx, h.tg, h.db, update.CallbackQuery)

		return
	}

	if update.Message == nil || update.Message.Text == "" {
		return
	}

	text := update.Message.Text

	switch {
	case isCommand(text, "start") || isCommand(text, "help"):
		handleHelp(ctx, h.tg, update.Message)
	case isCommand(text, "editsublist"):
		h.handleEditSublist(ctx, update.Message)
	}
}

func isCommand(text, cmd string) bool {
	return text == "/"+cmd ||
		strings.HasPrefix(text, "/"+cmd+" ") ||
		strings.HasPrefix(text, "/"+cmd+"@")
}
