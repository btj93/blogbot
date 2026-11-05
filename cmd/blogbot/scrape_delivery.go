package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/btj93/blogbot/config"
	"github.com/btj93/blogbot/model"
	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/scraper"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

type scrapeResult struct {
	blogs []model.Blog
	group string
	err   error
}

type scrapeStats struct {
	PostsFound, PostsSent, AlreadySeen                      int
	DisabledSkips, NoSubscriberSkips, LogOnlySkips          int
	RecipientsAccepted, RecipientsPartial, RecipientsFailed int
	EmptyGroups, Errors                                     int
}

func (s *scrapeStats) add(other scrapeStats) {
	s.PostsFound += other.PostsFound
	s.PostsSent += other.PostsSent
	s.AlreadySeen += other.AlreadySeen
	s.DisabledSkips += other.DisabledSkips
	s.NoSubscriberSkips += other.NoSubscriberSkips
	s.LogOnlySkips += other.LogOnlySkips
	s.RecipientsAccepted += other.RecipientsAccepted
	s.RecipientsPartial += other.RecipientsPartial
	s.RecipientsFailed += other.RecipientsFailed
	s.EmptyGroups += other.EmptyGroups
	s.Errors += other.Errors
}

func (s scrapeStats) attrs() []slog.Attr {
	return []slog.Attr{
		slog.Int("posts_found", s.PostsFound), slog.Int("posts_sent", s.PostsSent),
		slog.Int("already_seen", s.AlreadySeen), slog.Int("disabled_skips", s.DisabledSkips),
		slog.Int("no_subscriber_skips", s.NoSubscriberSkips), slog.Int("log_only_skips", s.LogOnlySkips),
		slog.Int("recipients_accepted", s.RecipientsAccepted), slog.Int("recipients_partial", s.RecipientsPartial),
		slog.Int("recipients_failed", s.RecipientsFailed), slog.Int("empty_groups", s.EmptyGroups),
		slog.Int("errors", s.Errors),
	}
}

func scrapeGroup(ctx context.Context, source scraper.Scraper, group string) scrapeResult {
	ctx = observability.WithAttrs(ctx, slog.String("group", group))
	start := time.Now()

	blogs, err := source.Scrape(ctx)
	switch {
	case err != nil:
		slog.ErrorContext(ctx, "scrape failed", slog.Any("error", err))
		observability.ScrapeErrorsTotal.Add(1)
		observability.PromScrapeErrorsTotal.WithLabelValues(group).Inc()
	case len(blogs) == 0:
		slog.WarnContext(ctx, "scrape returned no posts", slog.Int("parsed_posts", 0),
			slog.Duration("duration", time.Since(start)))
	default:
		slog.InfoContext(ctx, "group scrape completed", slog.Int("parsed_posts", len(blogs)),
			slog.Duration("duration", time.Since(start)))
	}

	return scrapeResult{blogs: blogs, group: group, err: err}
}

func processBlogs(
	ctx context.Context,
	db store.Querier,
	tg telegram.Sender,
	cfg config.ScraperConfig,
	blogs []model.Blog,
	group string,
) scrapeStats {
	ctx = observability.WithAttrs(ctx, slog.String("group", group))

	var stats scrapeStats
	defer func() {
		attrs := append([]slog.Attr{slog.Int("parsed_posts", len(blogs))}, stats.attrs()...)
		slog.LogAttrs(ctx, slog.LevelInfo, "group processing completed", attrs...)
	}()

	if len(blogs) == 0 {
		stats.EmptyGroups++
		return stats
	}

	grp, err := db.GetOrCreateGroup(ctx, group)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get group", slog.Any("error", err))

		stats.Errors++

		return stats
	}

	for _, blog := range blogs {
		postCtx := observability.WithAttrs(ctx, slog.String("blog_url", blog.URL), slog.String("member_name", blog.Name))
		if !blog.PublishedAt.IsZero() {
			postCtx = observability.WithAttrs(postCtx, slog.Time("published_at", blog.PublishedAt))
		}

		claimed, err := db.ClaimBlog(postCtx, blog.URL)
		if err != nil {
			slog.ErrorContext(postCtx, "failed to claim blog", slog.Any("error", err))

			stats.Errors++

			continue
		}

		if !claimed {
			stats.AlreadySeen++
			continue
		}

		stats.PostsFound++

		observability.ScrapePostsFound.Add(1)
		observability.PromScrapePostsFound.WithLabelValues(group).Inc()
		slog.InfoContext(postCtx, "new blog post", slog.String("title", blog.Title))
		tg.LogText(postCtx, fmt.Sprintf("%s %s\n%s", blog.Title, blog.Name, blog.URL))

		member, err := db.GetOrCreateMember(postCtx, grp.ID, blog.Name, nil, slices.Contains(cfg.StaffNames, blog.Name))
		if err != nil {
			slog.ErrorContext(postCtx, "failed to create member", slog.Any("error", err))

			stats.Errors++

			continue
		}

		postCtx = observability.WithAttrs(postCtx, slog.Int64("member_id", member.ID))
		if member.Disabled {
			stats.DisabledSkips++

			slog.InfoContext(postCtx, "post notification skipped", slog.String("reason", "member_disabled"))

			continue
		}

		images, err := scraper.DownloadImages(postCtx, blog.ImageURLs, cfg.UserAgent, nil)
		if err != nil || len(images) != len(blog.ImageURLs) {
			slog.WarnContext(postCtx, "blog image downloads incomplete", slog.Int("images_expected", len(blog.ImageURLs)),
				slog.Int("images_downloaded", len(images)), slog.Any("error", err))

			stats.Errors++
		}

		batches := scraper.BatchImagesGrouped(images, 10)

		msgText := fmt.Sprintf("#%s 「%s」\n\n%s", blog.Name, blog.Title, blog.URL)

		err = tg.SendText(postCtx, tg.LogChatID(), msgText, &telegram.SendTextOpts{DisableWebPagePreview: true})
		if err != nil {
			slog.ErrorContext(postCtx, "failed to send to log chat", slog.Any("error", err))

			stats.Errors++
		}

		for _, batch := range batches {
			if err := tg.SendMediaGroup(postCtx, tg.LogChatID(), batch); err != nil {
				slog.ErrorContext(postCtx, "failed to send media group to log chat", slog.Any("error", err))

				stats.Errors++
			}
		}

		if cfg.LogOnly {
			stats.LogOnlySkips++

			slog.InfoContext(postCtx, "post notification skipped", slog.String("reason", "log_only"))

			continue
		}

		chatIDs, err := db.GetSubscriberChatIDs(postCtx, member.ID)
		if err != nil {
			slog.ErrorContext(postCtx, "failed to get subscribers", slog.Any("error", err))

			stats.Errors++

			continue
		}

		slog.InfoContext(postCtx, "post subscribers loaded", slog.Int("subscriber_count", len(chatIDs)))

		if len(chatIDs) == 0 {
			stats.NoSubscriberSkips++

			slog.InfoContext(postCtx, "post notification skipped", slog.String("reason", "no_subscribers"))

			continue
		}

		before := stats

		for _, chatID := range chatIDs {
			cid, err := strconv.ParseInt(chatID, 10, 64)
			if err != nil || cid == 0 {
				slog.ErrorContext(postCtx, "invalid subscriber chat ID", slog.String("chat_id", chatID))

				stats.RecipientsFailed++
				stats.Errors++

				continue
			}

			result, errors := sendSubscriberNotification(
				postCtx,
				tg,
				cid,
				msgText,
				batches,
				len(images) == len(blog.ImageURLs),
			)
			stats.Errors += errors

			switch result {
			case "accepted":
				stats.RecipientsAccepted++

				tg.LogText(postCtx, fmt.Sprintf("sent #%s 「%s」 to %s", blog.Name, blog.Title, chatID))
			case "partial":
				stats.RecipientsPartial++
			case "failed":
				stats.RecipientsFailed++
			}
		}

		if stats.RecipientsAccepted > before.RecipientsAccepted {
			stats.PostsSent++
		}

		slog.InfoContext(postCtx, "post processing completed", slog.Int("subscriber_count", len(chatIDs)),
			slog.Int("recipients_accepted", stats.RecipientsAccepted-before.RecipientsAccepted),
			slog.Int("recipients_partial", stats.RecipientsPartial-before.RecipientsPartial),
			slog.Int("recipients_failed", stats.RecipientsFailed-before.RecipientsFailed))
	}

	return stats
}

func sendSubscriberNotification(
	ctx context.Context,
	tg telegram.Sender,
	chatID int64,
	text string,
	batches [][][]byte,
	imagesComplete bool,
) (string, int) {
	textAccepted := false

	var acceptedBatches, failures int

	if err := tg.SendText(ctx, chatID, text, &telegram.SendTextOpts{DisableWebPagePreview: true}); err != nil {
		slog.ErrorContext(ctx, "notification text failed", slog.Int64("chat_id", chatID), slog.Any("error", err))

		failures++
	} else {
		textAccepted = true
	}

	for i, batch := range batches {
		if err := tg.SendMediaGroup(ctx, chatID, batch); err != nil {
			slog.ErrorContext(ctx, "notification media failed", slog.Int64("chat_id", chatID),
				slog.Int("batch", i+1), slog.Any("error", err))

			failures++
		} else {
			acceptedBatches++
		}
	}

	result := "accepted"
	if failures > 0 || !imagesComplete {
		result = "failed"
		if textAccepted || acceptedBatches > 0 {
			result = "partial"
		}
	}

	slog.InfoContext(ctx, "notification attempt completed", slog.Int64("chat_id", chatID), slog.String("result", result),
		slog.Bool("text_accepted", textAccepted), slog.Int("media_batches", len(batches)),
		slog.Int("media_batches_accepted", acceptedBatches), slog.Bool("images_complete", imagesComplete))

	return result, failures
}
