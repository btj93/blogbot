package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/btj93/blogbot/bot"
	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Long-running: HTTP server for Telegram webhooks",
	RunE:  runWebhook,
}

func init() {
	rootCmd.AddCommand(webhookCmd)
}

func runWebhook(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	tg, err := telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.LogChatID, 5)
	if err != nil {
		return fmt.Errorf("creating telegram client: %w", err)
	}

	ctx := context.Background()
	ctx = observability.WithRunID(ctx, "webhook")

	handler := bot.NewHandler(tg, db, cfg.Telegram.BotToken, cfg.Webhook.WebAppURL, cfg.Webhook.UseWebApp)

	if cfg.Webhook.UseWebApp && cfg.Webhook.WebAppURL != "" {
		if err := tg.SetChatMenuButton(ctx, cfg.Webhook.WebAppURL, "訂閱管理"); err != nil {
			slog.ErrorContext(ctx, "failed to set menu button", slog.Any("error", err))
		} else {
			slog.InfoContext(ctx, "menu button set to WebApp")
		}
	} else {
		if err := tg.ResetChatMenuButton(ctx); err != nil {
			slog.ErrorContext(ctx, "failed to reset menu button", slog.Any("error", err))
		}
	}

	path := "/" + cfg.Telegram.BotToken
	http.Handle(path, handler)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/tg/blogbot/api/v1/members", handler.HandleMembers)
	http.HandleFunc("/tg/blogbot/api/v1/subscriptions", handler.HandleSubscriptions)
	http.HandleFunc("/tg/blogbot/api/v1/lock", handler.HandleLock)
	http.HandleFunc("/tg/blogbot/api/v1/member-images", bot.HandleMemberImages)

	// Serve the WebApp frontend static files.
	webappDir := cfg.Webhook.WebAppDir
	if webappDir != "" {
		fs := http.FileServer(http.Dir(webappDir))
		http.Handle("/tg/blogbot/", http.StripPrefix("/tg/blogbot/", fs))
	}

	slog.InfoContext(ctx, "webhook server listening", slog.String("addr", cfg.Webhook.ListenAddr))
	tg.LogText(ctx, fmt.Sprintf("Webhook server listening on %s", cfg.Webhook.ListenAddr))

	srv := &http.Server{
		Addr:         cfg.Webhook.ListenAddr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if cfg.Webhook.TLSCert != "" && cfg.Webhook.TLSKey != "" {
		slog.InfoContext(ctx, "TLS enabled", slog.String("cert", cfg.Webhook.TLSCert))
		tg.LogText(ctx, fmt.Sprintf("TLS enabled, cert: %s", cfg.Webhook.TLSCert))

		return srv.ListenAndServeTLS(cfg.Webhook.TLSCert, cfg.Webhook.TLSKey)
	}

	return srv.ListenAndServe()
}
