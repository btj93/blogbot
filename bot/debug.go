package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// The pinned Telegram SDK predates forward_origin, so decode that field separately.
type debugForward struct {
	Origin *debugOrigin  `json:"forward_origin"`
	Reply  *debugForward `json:"reply_to_message"`
}

type debugOrigin struct {
	Type       string         `json:"type"`
	SenderUser *tgbotapi.User `json:"sender_user"`
	SenderChat *tgbotapi.Chat `json:"sender_chat"`
	Chat       *tgbotapi.Chat `json:"chat"`
}

func (h *Handler) handleDebugForward(ctx context.Context, msg *tgbotapi.Message, body []byte) bool {
	ownerID := h.tg.LogChatID()
	if ownerID <= 0 || msg == nil || msg.Chat == nil || msg.From == nil ||
		msg.Chat.Type != "private" || msg.Chat.ID != ownerID || msg.From.ID != ownerID {
		return false
	}

	var envelope struct {
		Message debugForward `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		slog.ErrorContext(ctx, "failed to decode debug forward", slog.Any("error", err))
		return true
	}

	meta := &envelope.Message
	// Forwarded command text is data, not an instruction to inspect its reply.
	if isCommand(msg.Text, "debug") && meta.Origin == nil && msg.ForwardDate == 0 {
		if msg.ReplyToMessage == nil {
			h.sendDebug(ctx, "Forward a message here, or reply to a forwarded message with /debug.")
			return true
		}

		msg = msg.ReplyToMessage
		meta = meta.Reply
	}

	var id int64

	kind := "User ID / private chat ID"

	switch {
	case meta != nil && meta.Origin != nil:
		origin := meta.Origin
		switch {
		case origin.Type == "user" && origin.SenderUser != nil:
			id = origin.SenderUser.ID
		case origin.Type == "chat" && origin.SenderChat != nil:
			id, kind = origin.SenderChat.ID, "Origin chat ID"
		case origin.Type == "channel" && origin.Chat != nil:
			id, kind = origin.Chat.ID, "Origin channel ID"
		}
	case msg.ForwardFromChat != nil:
		id, kind = msg.ForwardFromChat.ID, "Origin chat ID"
	case msg.ForwardFrom != nil:
		id = msg.ForwardFrom.ID
	case msg.ForwardSenderName == "" && msg.ForwardDate == 0:
		return false
	}

	if id == 0 {
		h.sendDebug(
			ctx,
			"Telegram did not expose the original sender/chat ID (forwarding privacy may hide it). Cannot check subscriptions from a display name. Ask the sender to allow forwarded-message attribution, then forward a new message.",
		)

		return true
	}

	header := fmt.Sprintf("%s: %d\n", kind, id)

	ids, err := h.db.ListSubscribedMemberIDsForChat(ctx, strconv.FormatInt(id, 10))
	if err != nil {
		slog.ErrorContext(ctx, "debug subscription lookup failed", slog.Int64("chat_id", id), slog.Any("error", err))
		h.sendDebug(ctx, header+"Subscription lookup failed; status is unknown (not zero subscriptions).")

		return true
	}

	lines := make([]string, 0, len(ids))

	var disabled int

	for _, memberID := range ids {
		member, err := h.db.GetMemberByID(ctx, memberID)
		if err != nil {
			slog.ErrorContext(ctx, "debug member lookup failed", slog.Int64("member_id", memberID), slog.Any("error", err))
			h.sendDebug(ctx, header+"Subscription details lookup failed; results are incomplete.")

			return true
		}

		line := member.Name
		if member.Disabled {
			disabled++
			line += " [disabled: no notifications]"
		}

		lines = append(lines, line)
	}

	text := header + fmt.Sprintf(
		"Saved subscriptions: %d (%d enabled, %d disabled)\n",
		len(ids),
		len(ids)-disabled,
		disabled,
	)
	if len(ids) == 0 {
		text += "No saved subscriptions for this ID.\n"
	} else {
		text += strings.Join(lines, "\n") + "\n"
	}

	text += "Saved subscriptions do not prove Telegram delivered messages."
	if id > 0 {
		text += "\nThis checks private-chat subscriptions only. An ordinary forwarded user message does not reveal the source group ID or that group's subscriptions."
	}

	h.sendDebug(ctx, text)

	return true
}

func (h *Handler) sendDebug(ctx context.Context, text string) {
	// Stay below Telegram's message limit even when many members are subscribed.
	for remaining := []rune(text); len(remaining) > 0; {
		n := min(len(remaining), 3500)
		if err := h.tg.SendText(ctx, h.tg.LogChatID(), string(remaining[:n]), nil); err != nil {
			slog.ErrorContext(ctx, "failed to send debug result", slog.Any("error", err))
			return
		}

		remaining = remaining[n:]
	}
}
