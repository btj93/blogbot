package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

var adminCache struct {
	mu    sync.Mutex
	cache map[int64]adminEntry
}

type adminEntry struct {
	admins    map[int64]bool
	expiresAt time.Time
}

func init() {
	adminCache.cache = make(map[int64]adminEntry)
}

func checkPermission(ctx context.Context, tg telegram.Sender, chatID int64, userID int64) bool {
	if chatID == userID {
		return true
	}

	adminCache.mu.Lock()
	entry, ok := adminCache.cache[chatID]
	adminCache.mu.Unlock()

	if ok && time.Now().Before(entry.expiresAt) {
		return entry.admins[userID]
	}

	admins, err := tg.GetChatAdministrators(ctx, chatID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get admins", slog.Int64("chat_id", chatID), slog.Any("error", err))
		return false
	}

	adminMap := make(map[int64]bool)
	for _, a := range admins {
		adminMap[a.User.ID] = true
	}

	adminCache.mu.Lock()
	adminCache.cache[chatID] = adminEntry{admins: adminMap, expiresAt: time.Now().Add(5 * time.Minute)}
	adminCache.mu.Unlock()

	return adminMap[userID]
}

func handleCallback(
	ctx context.Context,
	tg telegram.Sender,
	db store.Querier,
	callback *tgbotapi.CallbackQuery,
) {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	userID := callback.From.ID
	data := callback.Data

	if !checkPermission(ctx, tg, chatID, userID) {
		tg.LogText(ctx, fmt.Sprintf("REJECTED\nGroup name: %s\nUser: %s %s @%s\nCallback: %s",
			callback.Message.Chat.Title, callback.From.FirstName, callback.From.LastName, callback.From.UserName, data))

		return
	}

	tg.LogText(ctx, fmt.Sprintf("Group name: %s\nUser: %s %s @%s\nCallback: %s",
		callback.Message.Chat.Title, callback.From.FirstName, callback.From.LastName, callback.From.UserName, data))

	chatIDStr := strconv.FormatInt(chatID, 10)

	switch {
	case strings.HasPrefix(data, "g:"):
		groupID, _ := strconv.ParseInt(data[2:], 10, 64)

		kb, err := generationKeyboard(ctx, db, groupID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to build generation keyboard", slog.Any("error", err))
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}

	case strings.HasPrefix(data, "gen:"):
		parts := strings.SplitN(data[4:], ":", 2)
		if len(parts) != 2 {
			return
		}

		groupID, _ := strconv.ParseInt(parts[0], 10, 64)
		gen := parseGeneration(parts[1])

		kb, err := memberKeyboard(ctx, db, groupID, gen, chatIDStr)
		if err != nil {
			slog.ErrorContext(ctx, "failed to build member keyboard", slog.Any("error", err))
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}

	case strings.HasPrefix(data, "t:"):
		memberID, _ := strconv.ParseInt(data[2:], 10, 64)

		subscribed, _ := db.IsSubscribed(ctx, memberID, chatIDStr)
		if subscribed {
			if _, err := db.RemoveSubscription(ctx, memberID, chatIDStr); err != nil {
				slog.ErrorContext(
					ctx,
					"failed to remove subscription",
					slog.Int64("member_id", memberID),
					slog.Any("error", err),
				)
			}
		} else {
			if _, err := db.AddSubscription(ctx, memberID, chatIDStr); err != nil {
				slog.ErrorContext(
					ctx,
					"failed to add subscription",
					slog.Int64("member_id", memberID),
					slog.Any("error", err),
				)
			}
		}

		member, err := db.GetMemberByID(ctx, memberID)
		if err != nil {
			return
		}

		kb, err := memberKeyboard(ctx, db, member.GroupID, member.Generation, chatIDStr)
		if err != nil {
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}

	case strings.HasPrefix(data, "+all:"):
		parts := strings.SplitN(data[5:], ":", 2)
		if len(parts) != 2 {
			return
		}

		groupID, _ := strconv.ParseInt(parts[0], 10, 64)

		gen := parseGeneration(parts[1])
		if err := db.AddSubscriptionForAllInGeneration(ctx, groupID, gen, chatIDStr); err != nil {
			slog.ErrorContext(ctx, "failed to add all subscriptions in generation", slog.Any("error", err))
		}

		kb, err := memberKeyboard(ctx, db, groupID, gen, chatIDStr)
		if err != nil {
			slog.ErrorContext(ctx, "failed to build member keyboard", slog.Any("error", err))
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}

	case strings.HasPrefix(data, "-all:"):
		parts := strings.SplitN(data[5:], ":", 2)
		if len(parts) != 2 {
			return
		}

		groupID, _ := strconv.ParseInt(parts[0], 10, 64)

		gen := parseGeneration(parts[1])
		if err := db.RemoveSubscriptionForAllInGeneration(ctx, groupID, gen, chatIDStr); err != nil {
			slog.ErrorContext(ctx, "failed to remove all subscriptions in generation", slog.Any("error", err))
		}

		kb, err := memberKeyboard(ctx, db, groupID, gen, chatIDStr)
		if err != nil {
			slog.ErrorContext(ctx, "failed to build member keyboard", slog.Any("error", err))
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}

	case data == "back:groups":
		kb, err := groupKeyboard(ctx, db)
		if err != nil {
			slog.ErrorContext(ctx, "failed to build group keyboard", slog.Any("error", err))
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}

	case strings.HasPrefix(data, "back:gen:"):
		groupID, _ := strconv.ParseInt(data[9:], 10, 64)

		kb, err := generationKeyboard(ctx, db, groupID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to build generation keyboard", slog.Any("error", err))
			return
		}

		if err := tg.EditMessageTextAndMarkup(ctx, chatID, messageID, editSublistText, kb); err != nil {
			slog.ErrorContext(ctx, "failed to edit message", slog.Any("error", err))
		}
	}
}
