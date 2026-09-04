package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/showroom"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

var showroomNextliveCmd = &cobra.Command{
	Use:   "showroom-nextlive",
	Short: "One-shot: poll Showroom next_live API and notify on changes",
	RunE:  runShowroomNextlive,
}

func init() {
	rootCmd.AddCommand(showroomNextliveCmd)
}

func runShowroomNextlive(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx = observability.WithRunID(ctx, "showroom-nextlive")

	db, err := store.Open(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	tg, err := telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.LogChatID, 5)
	if err != nil {
		return fmt.Errorf("creating telegram client: %w", err)
	}

	observability.NextLiveChecksTotal.Add(1)
	observability.PromNextLiveChecksTotal.Inc()

	start := time.Now()

	defer func() { observability.PromNextLiveDuration.Observe(time.Since(start).Seconds()) }()

	slog.InfoContext(ctx, "start searching nextLive")
	tg.LogText(ctx, "started")

	checkErr := showroom.CheckNextLive(ctx, db, tg, cfg.Scraper.LogOnly)

	duration := time.Since(start).Truncate(time.Millisecond)
	slog.InfoContext(ctx, "showroom-nextlive completed",
		slog.Duration("duration", duration),
		slog.Any("error", checkErr),
	)
	tg.LogText(ctx, fmt.Sprintf(
		"completed\nduration: %s\nerror: %v",
		duration, checkErr,
	))

	if checkErr != nil {
		return fmt.Errorf("checking next live: %w", checkErr)
	}

	return nil
}
