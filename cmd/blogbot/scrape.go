package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/scraper"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "One-shot blog scrape for all groups",
	RunE:  runScrape,
}

func init() {
	rootCmd.AddCommand(scrapeCmd)
}

func runScrape(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx = observability.WithRunID(ctx, "scrape")

	db, err := store.Open(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	tg, err := telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.LogChatID, 5)
	if err != nil {
		return fmt.Errorf("creating telegram client: %w", err)
	}

	observability.ScrapeRunsTotal.Add(1)
	observability.PromScrapeRunsTotal.Inc()

	start := time.Now()

	defer func() { observability.PromScrapeDuration.Observe(time.Since(start).Seconds()) }()

	var totals scrapeStats

	slog.InfoContext(ctx, "start searching blogs")
	tg.LogText(ctx, "started")

	scrapers := []struct {
		scraper scraper.Scraper
		group   string
	}{
		{&scraper.NogiScraper{UserAgent: cfg.Scraper.UserAgent}, "乃木坂46"},
		{&scraper.SakuScraper{UserAgent: cfg.Scraper.UserAgent}, "櫻坂46"},
		{&scraper.HinaScraper{UserAgent: cfg.Scraper.UserAgent}, "日向坂46"},
	}

	g, gctx := errgroup.WithContext(ctx)
	results := make([]scrapeResult, len(scrapers))

	for i, s := range scrapers {
		g.Go(func() error {
			results[i] = scrapeGroup(gctx, s.scraper, s.group)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "scrape goroutine failed", slog.Any("error", err))
	}

	for _, result := range results {
		if result.err != nil {
			totals.Errors++
			continue
		}

		totals.add(processBlogs(ctx, db, tg, cfg.Scraper, result.blogs, result.group))
	}

	duration := time.Since(start).Truncate(time.Millisecond)
	attrs := append([]slog.Attr{slog.Duration("duration", duration)}, totals.attrs()...)
	slog.LogAttrs(ctx, slog.LevelInfo, "scrape completed", attrs...)
	tg.LogText(ctx, fmt.Sprintf(
		"completed\nduration: %s\nposts_found: %d\nposts_sent: %d\nrecipients_accepted: %d\nrecipients_partial: %d\nrecipients_failed: %d\nempty_groups: %d\nerrors: %d",
		duration,
		totals.PostsFound,
		totals.PostsSent,
		totals.RecipientsAccepted,
		totals.RecipientsPartial,
		totals.RecipientsFailed,
		totals.EmptyGroups,
		totals.Errors,
	))

	return nil
}
