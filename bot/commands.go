package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/telegram"
)

const (
	editSublistText = "請選擇心儀成員 選擇後即可接收到blog更新及showroom提示"

	helpTextGroup = "簡介:\n此bot提供乃木坂46, 櫻坂46, 日向坂成員blog更新及showroom提示\n使用者可依照個人喜好選擇接收(訂閱)哪些成員的資訊 (預設為無)\n私訊及群組對話皆可使用 (群組對話只有管理員可以修改訂閱列表)\n\n指令一覽:\n/help@NogiBlog_bot - 顯示此頁面\n/editsublist@NogiBlog_bot - 編輯訂閱列表\n\n如有任何問題請聯絡:https://t.me/btj93"

	helpTextPrivate = "簡介:\n此bot提供乃木坂46, 櫻坂46, 日向坂成員blog更新及showroom提示\n使用者可依照個人喜好選擇接收(訂閱)哪些成員的資訊 (預設為無)\n私訊及群組對話皆可使用 (群組對話只有管理員可以修改訂閱列表)\n\n指令一覽:\n/help - 顯示此頁面\n/editsublist - 編輯訂閱列表\n\n如有任何問題請聯絡:https://t.me/btj93"

	noPermissionText = "親 你沒有此權限哦"
)

func handleHelp(ctx context.Context, tg telegram.Sender, msg *tgbotapi.Message) {
	if msg.Chat.IsPrivate() {
		if err := tg.SendText(ctx, msg.Chat.ID, helpTextPrivate, nil); err != nil {
			slog.ErrorContext(ctx, "failed to send help text", slog.Any("error", err))
		}
	} else {
		if err := tg.SendText(ctx, msg.Chat.ID, helpTextGroup, nil); err != nil {
			slog.ErrorContext(ctx, "failed to send help text", slog.Any("error", err))
		}
	}
}

func (h *Handler) handleEditSublist(ctx context.Context, msg *tgbotapi.Message) {
	if !checkPermission(ctx, h.tg, msg.Chat.ID, msg.From.ID) {
		if err := h.tg.SendText(ctx, msg.Chat.ID, noPermissionText, nil); err != nil {
			slog.ErrorContext(ctx, "failed to send no-permission text", slog.Any("error", err))
		}

		return
	}

	if h.useWebApp {
		h.sendWebAppButton(ctx, msg)
		return
	}

	kb, err := groupKeyboard(ctx, h.db)
	if err != nil {
		return
	}

	if err := h.tg.SendText(ctx, msg.Chat.ID, editSublistText, &telegram.SendTextOpts{
		ReplyMarkup: kb,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to send editsublist text", slog.Any("error", err))
	}
}

func (h *Handler) sendWebAppButton(ctx context.Context, msg *tgbotapi.Message) {
	chatIDStr := strconv.FormatInt(msg.Chat.ID, 10)
	url := fmt.Sprintf("https://t.me/NogiBlog_bot/app?startapp=%s", chatIDStr)

	button := tgbotapi.InlineKeyboardButton{Text: "開啟訂閱管理", URL: &url}
	kb := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(button))

	if err := h.tg.SendText(ctx, msg.Chat.ID, editSublistText, &telegram.SendTextOpts{
		ReplyMarkup: kb,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to send webapp button", slog.Any("error", err))
	}
}
