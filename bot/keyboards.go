package bot

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/store"
)

func groupKeyboard(ctx context.Context, db store.Querier) (tgbotapi.InlineKeyboardMarkup, error) {
	groups, err := db.ListGroups(ctx)
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	var buttons []tgbotapi.InlineKeyboardButton
	for _, g := range groups {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(
			g.Name, fmt.Sprintf("g:%d", g.ID),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttons...),
	), nil
}

func generationKeyboard(ctx context.Context, db store.Querier, groupID int64) (tgbotapi.InlineKeyboardMarkup, error) {
	gens, err := db.ListGenerationsForGroup(ctx, groupID)
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	var (
		rows [][]tgbotapi.InlineKeyboardButton
		row  []tgbotapi.InlineKeyboardButton
	)

	for _, gen := range gens {
		var label, data string
		if gen == nil {
			label = "其他"
			data = fmt.Sprintf("gen:%d:null", groupID)
		} else {
			label = genLabel(*gen)
			data = fmt.Sprintf("gen:%d:%d", groupID, *gen)
		}

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, data))
		if len(row) == 3 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
			row = nil
		}
	}

	if len(row) > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("back", "back:groups"),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...), nil
}

func memberKeyboard(
	ctx context.Context,
	db store.Querier,
	groupID int64,
	generation *int,
	chatID string,
) (tgbotapi.InlineKeyboardMarkup, error) {
	members, err := db.ListEnabledMembersByGroupAndGeneration(ctx, groupID, generation)
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	var (
		rows [][]tgbotapi.InlineKeyboardButton
		row  []tgbotapi.InlineKeyboardButton
	)

	for _, m := range members {
		subscribed, _ := db.IsSubscribed(ctx, m.ID, chatID)

		label := m.Name
		if subscribed {
			label = "✓ " + label
		}

		data := fmt.Sprintf("t:%d", m.ID)

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, data))
		if len(row) == 3 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
			row = nil
		}
	}

	if len(row) > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	}

	genStr := "null"
	if generation != nil {
		genStr = strconv.Itoa(*generation)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("+all", fmt.Sprintf("+all:%d:%s", groupID, genStr)),
		tgbotapi.NewInlineKeyboardButtonData("-all", fmt.Sprintf("-all:%d:%s", groupID, genStr)),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("back", fmt.Sprintf("back:gen:%d", groupID)),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...), nil
}

func genLabel(gen int) string {
	numerals := []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十"}
	if gen > 0 && gen < len(numerals) {
		return numerals[gen] + "期生"
	}

	return fmt.Sprintf("%d期生", gen)
}

func parseGeneration(s string) *int {
	if s == "null" {
		return nil
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}

	return &v
}
