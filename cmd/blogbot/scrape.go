package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/btj93/blogbot/model"
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

	var postsFound, postsSent, scrapeErrors int

	slog.InfoContext(ctx, "start searching blogs")
	tg.LogText(ctx, "started")

	type scrapeResult struct {
		blogs []model.Blog
		group string
	}

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
			blogs, err := s.scraper.Scrape(gctx)
			if err != nil {
				slog.ErrorContext(gctx, "scrape failed", slog.String("group", s.group), slog.Any("error", err))
				observability.ScrapeErrorsTotal.Add(1)
				observability.PromScrapeErrorsTotal.WithLabelValues(s.group).Inc()

				return nil // Don't fail entire run for one group
			}

			results[i] = scrapeResult{blogs: blogs, group: s.group}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "scrape goroutine failed", slog.Any("error", err))
	}

	for _, r := range results {
		if r.blogs == nil {
			continue
		}

		grp, err := db.GetOrCreateGroup(ctx, r.group)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get group", slog.String("group", r.group), slog.Any("error", err))

			scrapeErrors++

			continue
		}

		for _, blog := range r.blogs {
			claimed, err := db.ClaimBlog(ctx, blog.URL)
			if err != nil {
				slog.ErrorContext(ctx, "failed to claim blog", slog.String("url", blog.URL), slog.Any("error", err))

				scrapeErrors++

				continue
			}

			if !claimed {
				continue
			}

			slog.InfoContext(
				ctx,
				"new blog post",
				slog.String("title", blog.Title),
				slog.String("name", blog.Name),
				slog.String("url", blog.URL),
			)

			postsFound++

			observability.ScrapePostsFound.Add(1)
			observability.PromScrapePostsFound.WithLabelValues(r.group).Inc()
			tg.LogText(ctx, fmt.Sprintf("%s %s\n%s", blog.Title, blog.Name, blog.URL))

			// Auto-create member
			isStaff := slices.Contains(cfg.Scraper.StaffNames, blog.Name)

			member, err := db.GetOrCreateMember(ctx, grp.ID, blog.Name, nil, isStaff)
			if err != nil {
				slog.ErrorContext(ctx, "failed to create member", slog.Any("error", err))

				scrapeErrors++

				continue
			}

			if !member.Disabled {
				// Download images
				images, err := scraper.DownloadImages(ctx, blog.ImageURLs, cfg.Scraper.UserAgent, nil)
				if err != nil {
					slog.ErrorContext(ctx, "failed to download images", slog.Any("error", err))
				}

				msgText := fmt.Sprintf("#%s 「%s」\n\n%s", blog.Name, blog.Title, blog.URL)

				// Send to log chat
				if err := tg.SendText(
					ctx,
					tg.LogChatID(),
					msgText,
					&telegram.SendTextOpts{DisableWebPagePreview: true},
				); err != nil {
					slog.ErrorContext(ctx, "failed to send to log chat", slog.Any("error", err))
				}

				for _, batch := range scraper.BatchImagesGrouped(images, 10) {
					if err := tg.SendMediaGroup(ctx, tg.LogChatID(), batch); err != nil {
						slog.ErrorContext(ctx, "failed to send media group to log chat", slog.Any("error", err))
					}
				}

				// Send to subscribers (skip in log-only mode)
				if cfg.Scraper.LogOnly {
					slog.InfoContext(ctx, "log-only mode, skipping subscriber notifications",
						slog.String("name", blog.Name), slog.String("title", blog.Title))
				} else {
					chatIDs, err := db.GetSubscriberChatIDs(ctx, member.ID)
					if err != nil {
						slog.ErrorContext(ctx, "failed to get subscribers", slog.Any("error", err))
					}

					for _, chatID := range chatIDs {
						cid := parseChatID(chatID)
						if err := tg.SendText(ctx, cid, msgText, &telegram.SendTextOpts{DisableWebPagePreview: true}); err != nil {
							slog.ErrorContext(ctx, "failed to send to subscriber", slog.Any("error", err))
						}

						for _, batch := range scraper.BatchImagesGrouped(images, 10) {
							if err := tg.SendMediaGroup(ctx, cid, batch); err != nil {
								slog.ErrorContext(ctx, "failed to send media group to subscriber", slog.Any("error", err))
							}
						}

						tg.LogText(ctx, fmt.Sprintf("sent #%s 「%s」 to %s", blog.Name, blog.Title, chatID))
					}
				}
			}

			postsSent++
		}
	}

	duration := time.Since(start).Truncate(time.Millisecond)
	slog.InfoContext(ctx, "scrape completed",
		slog.Duration("duration", duration),
		slog.Int("posts_found", postsFound),
		slog.Int("posts_sent", postsSent),
		slog.Int("errors", scrapeErrors),
	)
	tg.LogText(ctx, fmt.Sprintf(
		"completed\nduration: %s\nposts_found: %d\nposts_sent: %d\nerrors: %d",
		duration, postsFound, postsSent, scrapeErrors,
	))

	return nil
}

func parseChatID(s string) int64 {
	var n int64

	_, _ = fmt.Sscanf(s, "%d", &n)

	return n
}
