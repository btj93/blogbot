package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/observability"
)

type Client struct {
	bot        *tgbotapi.BotAPI
	logChatID  int64
	maxRetries int
}

func NewClient(token string, logChatID string, maxRetries int) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating bot: %w", err)
	}

	lcid, _ := strconv.ParseInt(logChatID, 10, 64)

	if maxRetries <= 0 {
		maxRetries = 5
	}

	return &Client{bot: bot, logChatID: lcid, maxRetries: maxRetries}, nil
}

func (c *Client) Bot() *tgbotapi.BotAPI {
	return c.bot
}

func (c *Client) LogChatID() int64 {
	return c.logChatID
}

type SendTextOpts struct {
	DisableWebPagePreview bool
	ReplyMarkup           any
}

func (c *Client) SendText(ctx context.Context, chatID int64, text string, opts *SendTextOpts) error {
	ctx = observability.WithAttrs(ctx, slog.Int64("chat_id", chatID), slog.String("telegram_method", "sendMessage"))

	msg := tgbotapi.NewMessage(chatID, text)
	if opts != nil {
		msg.DisableWebPagePreview = opts.DisableWebPagePreview
		if opts.ReplyMarkup != nil {
			msg.ReplyMarkup = opts.ReplyMarkup
		}
	}

	return c.sendWithRetry(ctx, func() error {
		sent, err := c.bot.Send(msg)
		if err == nil {
			slog.InfoContext(ctx, "telegram send accepted", slog.Int("message_id", sent.MessageID))
		}

		return err
	}, fmt.Sprintf("SendText to %d", chatID))
}

func (c *Client) SendMediaGroup(ctx context.Context, chatID int64, photos [][]byte) error {
	if len(photos) == 0 {
		return nil
	}

	ctx = observability.WithAttrs(ctx, slog.Int64("chat_id", chatID), slog.String("telegram_method", "sendMediaGroup"))

	var media []any

	for i, p := range photos {
		fileBytes := tgbotapi.FileBytes{Name: fmt.Sprintf("photo_%d.jpg", i), Bytes: p}
		media = append(media, tgbotapi.NewInputMediaPhoto(fileBytes))
	}

	cfg := tgbotapi.NewMediaGroup(chatID, media)

	return c.sendWithRetry(ctx, func() error {
		sent, err := c.bot.SendMediaGroup(cfg)
		if err == nil {
			ids := make([]int, 0, len(sent))
			for _, message := range sent {
				ids = append(ids, message.MessageID)
			}

			slog.InfoContext(ctx, "telegram send accepted", slog.Any("message_ids", ids))
		}

		return err
	}, fmt.Sprintf("SendMediaGroup to %d", chatID))
}

func (c *Client) sendWithRetry(ctx context.Context, fn func() error, desc string) error {
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		delay, shouldRetry := c.parseRetryError(err)
		if attempt == c.maxRetries || !shouldRetry {
			observability.TelegramSendErrors.Add(1)
			observability.PromTelegramSendErrors.Inc()

			var code int

			apiErr := &tgbotapi.Error{}
			if errors.As(err, &apiErr) {
				code = apiErr.Code
			}

			slog.ErrorContext(ctx, "telegram send failed", slog.Int("attempts", attempt+1),
				slog.Int("telegram_error_code", code), slog.Any("error", err))

			if attempt == c.maxRetries {
				return fmt.Errorf("%s failed after %d retries: %w", desc, c.maxRetries, err)
			}

			return fmt.Errorf("%s: %w", desc, err)
		}

		slog.WarnContext(ctx, "telegram send retry", slog.Int("attempt", attempt+1),
			slog.Duration("retry_after", delay), slog.Any("error", err))
		time.Sleep(delay)
		observability.TelegramRetries.Add(1)
		observability.PromTelegramRetries.Inc()
	}

	return nil // unreachable
}

func (c *Client) parseRetryError(err error) (time.Duration, bool) {
	apiErr := &tgbotapi.Error{}
	if errors.As(err, &apiErr) {
		if apiErr.Code == 429 {
			var params struct {
				RetryAfter int `json:"retry_after"`
			}
			if json.Unmarshal([]byte(apiErr.Message), &params) == nil && params.RetryAfter > 0 {
				return time.Duration(params.RetryAfter+5) * time.Second, true
			}

			return 30 * time.Second, true
		}

		if apiErr.Code == 400 && strings.Contains(apiErr.Message, "group send failed") {
			return 5 * time.Second, true
		}
	}

	return 0, false
}

func (c *Client) EditMessageTextAndMarkup(
	_ context.Context,
	chatID int64,
	messageID int,
	text string,
	markup tgbotapi.InlineKeyboardMarkup,
) error {
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, markup)
	_, err := c.bot.Send(editMsg)

	return err
}

func (c *Client) GetChatAdministrators(_ context.Context, chatID int64) ([]tgbotapi.ChatMember, error) {
	cfg := tgbotapi.ChatAdministratorsConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: chatID}}
	return c.bot.GetChatAdministrators(cfg)
}

func (c *Client) GetChat(_ context.Context, chatID int64) (tgbotapi.Chat, error) {
	return c.bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: chatID}})
}

func (c *Client) AnswerCallbackQuery(_ context.Context, callbackQueryID, text string) error {
	cb := tgbotapi.NewCallback(callbackQueryID, text)
	_, err := c.bot.Request(cb)

	return err
}

type menuButtonWebApp struct {
	Type   string     `json:"type"`
	Text   string     `json:"text"`
	WebApp webAppInfo `json:"web_app"`
}

type webAppInfo struct {
	URL string `json:"url"`
}

func (c *Client) SetChatMenuButton(_ context.Context, url, text string) error {
	mb := menuButtonWebApp{
		Type:   "web_app",
		Text:   text,
		WebApp: webAppInfo{URL: url},
	}

	b, err := json.Marshal(mb)
	if err != nil {
		return fmt.Errorf("marshaling menu button: %w", err)
	}

	params := tgbotapi.Params{}
	params["menu_button"] = string(b)
	_, err = c.bot.MakeRequest("setChatMenuButton", params)

	return err
}

func (c *Client) ResetChatMenuButton(_ context.Context) error {
	params := tgbotapi.Params{}
	params["menu_button"] = `{"type":"default"}`
	_, err := c.bot.MakeRequest("setChatMenuButton", params)

	return err
}

func (c *Client) LogText(ctx context.Context, text string) {
	text = observability.RedactTelegramTokens(formatLogText(ctx, text))

	if c.logChatID != 0 {
		if err := c.SendText(ctx, c.logChatID, text, &SendTextOpts{DisableWebPagePreview: true}); err != nil {
			slog.ErrorContext(ctx, "failed to send log text", slog.Any("error", err))
		}
	} else {
		slog.InfoContext(ctx, "log message", slog.String("text", text))
	}
}

func formatLogText(ctx context.Context, text string) string {
	if cmd := observability.Command(ctx); cmd != "" {
		text = fmt.Sprintf("[%s] %s", cmd, text)
	}

	if runID := observability.RunID(ctx); runID != "" {
		text = fmt.Sprintf("%s\nrun_id: %s", text, runID)
	}

	return text
}
