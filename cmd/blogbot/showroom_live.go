package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/showroom"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

var showroomLiveCmd = &cobra.Command{
	Use:   "showroom-live",
	Short: "Long-running: WebSocket live detection per member",
	RunE:  runShowroomLive,
}

func init() {
	rootCmd.AddCommand(showroomLiveCmd)
}

func runShowroomLive(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx = observability.WithRunID(ctx, "showroom-live")

	db, err := store.Open(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	tg, err := telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.LogChatID, 5)
	if err != nil {
		return fmt.Errorf("creating telegram client: %w", err)
	}

	slog.InfoContext(ctx, "starting showroom live monitor")
	tg.LogText(ctx, "Starting Showroom live monitor")

	err = showroom.MonitorLive(ctx, db, tg, cfg.Scraper.LogOnly)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	slog.InfoContext(ctx, "showroom live monitor stopped")
	tg.LogText(ctx, "Showroom live monitor stopped")

	return nil
}
