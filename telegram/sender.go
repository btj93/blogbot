package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender abstracts Telegram message sending for testability.
type Sender interface {
	SendText(ctx context.Context, chatID int64, text string, opts *SendTextOpts) error
	SendMediaGroup(ctx context.Context, chatID int64, photos [][]byte) error
	LogText(ctx context.Context, text string)
	LogChatID() int64
	// EditMessageTextAndMarkup edits an existing message's text and inline keyboard.
	EditMessageTextAndMarkup(
		ctx context.Context,
		chatID int64,
		messageID int,
		text string,
		markup tgbotapi.InlineKeyboardMarkup,
	) error
	// GetChatAdministrators returns the list of administrators for a chat.
	GetChatAdministrators(ctx context.Context, chatID int64) ([]tgbotapi.ChatMember, error)
	// GetChat returns chat info (title, type, etc).
	GetChat(ctx context.Context, chatID int64) (tgbotapi.Chat, error)
	// AnswerCallbackQuery sends an answer to a callback query (toast notification).
	AnswerCallbackQuery(ctx context.Context, callbackQueryID, text string) error
	// SetChatMenuButton sets the bot's default menu button to open a WebApp.
	SetChatMenuButton(ctx context.Context, url, text string) error
	// ResetChatMenuButton resets the bot's menu button to the default command menu.
	ResetChatMenuButton(ctx context.Context) error
}
