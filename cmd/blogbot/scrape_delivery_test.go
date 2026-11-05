package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btj93/blogbot/config"
	"github.com/btj93/blogbot/model"
	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

func captureScrapeLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(
		slog.New(
			observability.NewContextHandler(
				slog.NewJSONHandler(&logs, nil),
				[]observability.ContextKey{observability.RunIDKey{}},
			),
		),
	)
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &logs
}

type deliveryDB struct {
	store.Querier
	disabled, alreadySeen bool
	chats                 []string
	subscriberErr         error
}

func (*deliveryDB) GetOrCreateGroup(context.Context, string) (*model.Group, error) {
	return &model.Group{ID: 1}, nil
}
func (db *deliveryDB) ClaimBlog(context.Context, string) (bool, error) { return !db.alreadySeen, nil }
func (db *deliveryDB) GetOrCreateMember(context.Context, int64, string, *int, bool) (*model.Member, error) {
	return &model.Member{ID: 7, Disabled: db.disabled}, nil
}
func (db *deliveryDB) GetSubscriberChatIDs(context.Context, int64) ([]string, error) {
	return db.chats, db.subscriberErr
}

type deliverySender struct {
	telegram.Sender
	failText, failMedia map[int64]bool
	subscriberCalls     int
	logTexts            []string
}

func (*deliverySender) LogChatID() int64 { return 999 }
func (s *deliverySender) LogText(_ context.Context, text string) {
	s.logTexts = append(s.logTexts, text)
}
func (s *deliverySender) SendText(_ context.Context, chatID int64, _ string, _ *telegram.SendTextOpts) error {
	if chatID != 999 {
		s.subscriberCalls++
	}

	if s.failText[chatID] {
		return errors.New("text rejected")
	}

	return nil
}
func (s *deliverySender) SendMediaGroup(_ context.Context, chatID int64, _ [][]byte) error {
	if chatID != 999 {
		s.subscriberCalls++
	}

	if s.failMedia[chatID] {
		return errors.New("media rejected")
	}

	return nil
}

func TestProcessBlogsDeliveryAccounting(t *testing.T) {
	for _, tt := range []struct {
		name                                                  string
		disabled, alreadySeen, logOnly, images, missingImages bool
		chats                                                 []string
		failText, failMedia                                   map[int64]bool
		subscriberErr                                         error
		want                                                  scrapeStats
		wantResult                                            string
	}{
		{name: "disabled", disabled: true, chats: []string{"42"}, want: scrapeStats{PostsFound: 1, DisabledSkips: 1}},
		{name: "no subscribers", want: scrapeStats{PostsFound: 1, NoSubscriberSkips: 1}},
		{name: "log only", logOnly: true, chats: []string{"42"}, want: scrapeStats{PostsFound: 1, LogOnlySkips: 1}},
		{name: "already seen", alreadySeen: true, want: scrapeStats{AlreadySeen: 1}},
		{name: "accepted", chats: []string{"42"}, images: true, want: scrapeStats{PostsFound: 1, PostsSent: 1, RecipientsAccepted: 1}, wantResult: "accepted"},
		{name: "text only accepted", chats: []string{"42"}, want: scrapeStats{PostsFound: 1, PostsSent: 1, RecipientsAccepted: 1}, wantResult: "accepted"},
		{name: "all parts fail", chats: []string{"42"}, images: true, failText: map[int64]bool{42: true}, failMedia: map[int64]bool{42: true}, want: scrapeStats{PostsFound: 1, RecipientsFailed: 1, Errors: 2}, wantResult: "failed"},
		{name: "text only fails", chats: []string{"42"}, failText: map[int64]bool{42: true}, want: scrapeStats{PostsFound: 1, RecipientsFailed: 1, Errors: 1}, wantResult: "failed"},
		{name: "text fails media accepted", chats: []string{"42"}, images: true, failText: map[int64]bool{42: true}, want: scrapeStats{PostsFound: 1, RecipientsPartial: 1, Errors: 1}, wantResult: "partial"},
		{name: "media fails text accepted", chats: []string{"42"}, images: true, failMedia: map[int64]bool{42: true}, want: scrapeStats{PostsFound: 1, RecipientsPartial: 1, Errors: 1}, wantResult: "partial"},
		{name: "missing download is partial", chats: []string{"42"}, images: true, missingImages: true, want: scrapeStats{PostsFound: 1, RecipientsPartial: 1, Errors: 1}, wantResult: "partial"},
		{name: "mixed recipients", chats: []string{"42", "43"}, failText: map[int64]bool{42: true}, want: scrapeStats{PostsFound: 1, PostsSent: 1, RecipientsAccepted: 1, RecipientsFailed: 1, Errors: 1}},
		{name: "invalid ID", chats: []string{"garbage"}, want: scrapeStats{PostsFound: 1, RecipientsFailed: 1, Errors: 1}},
		{name: "subscriber lookup fails", subscriberErr: errors.New("database offline"), want: scrapeStats{PostsFound: 1, Errors: 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureScrapeLogs(t)
			blog := model.Blog{Name: "member", Title: "post", URL: "https://example.com/post/1"}

			if tt.images {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if tt.missingImages {
						w.WriteHeader(http.StatusNotFound)
						return
					}

					_, _ = io.WriteString(w, "image bytes")
				}))
				t.Cleanup(server.Close)
				blog.ImageURLs = []string{server.URL + "/image.jpg"}
			}

			db := &deliveryDB{
				disabled:      tt.disabled,
				alreadySeen:   tt.alreadySeen,
				chats:         tt.chats,
				subscriberErr: tt.subscriberErr,
			}
			tg := &deliverySender{failText: tt.failText, failMedia: tt.failMedia}
			ctx := observability.WithRunID(context.Background(), "scrape")

			got := processBlogs(ctx, db, tg, config.ScraperConfig{LogOnly: tt.logOnly}, []model.Blog{blog}, "test group")
			if got != tt.want {
				t.Fatalf("stats=%+v, want %+v\nlogs: %s", got, tt.want, logs.String())
			}

			if tt.wantResult != "" {
				for _, field := range []string{`"result":"` + tt.wantResult + `"`, `"chat_id":42`, `"member_id":7`, `"blog_url":"https://example.com/post/1"`, `"run_id":`, `"group":"test group"`} {
					if !strings.Contains(logs.String(), field) {
						t.Errorf("missing %s: %s", field, logs.String())
					}
				}
			} else if tt.want.DisabledSkips+tt.want.NoSubscriberSkips+tt.want.LogOnlySkips+tt.want.AlreadySeen > 0 && tg.subscriberCalls != 0 {
				t.Fatal("skipped post attempted subscriber sends")
			}

			for _, text := range tg.logTexts {
				if strings.HasPrefix(text, "sent #") {
					t.Fatal("per-recipient detail leaked to Telegram log chat")
				}
			}

			if !strings.Contains(logs.String(), `"msg":"group processing completed"`) {
				t.Fatal("missing group summary")
			}
		})
	}
}

type diagnosticScraper struct {
	blogs []model.Blog
	err   error
}

func (s diagnosticScraper) Scrape(context.Context) ([]model.Blog, error) { return s.blogs, s.err }

func TestScrapeGroupDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  diagnosticScraper
		message string
		wantErr bool
	}{
		{name: "empty", message: "scrape returned no posts"},
		{name: "parsed", source: diagnosticScraper{blogs: []model.Blog{{URL: "https://example.com/post"}}}, message: "group scrape completed"},
		{name: "failure", source: diagnosticScraper{err: errors.New("HTTP 502")}, message: "scrape failed", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureScrapeLogs(t)

			result := scrapeGroup(observability.WithRunID(context.Background(), "scrape"), tt.source, "test group")
			if (result.err != nil) != tt.wantErr || result.group != "test group" {
				t.Fatalf("unexpected result: %+v", result)
			}

			if !strings.Contains(logs.String(), tt.message) || !strings.Contains(logs.String(), `"group":"test group"`) ||
				!strings.Contains(logs.String(), `"run_id":`) {
				t.Fatalf("missing scraper diagnostic: %s", logs.String())
			}

			if tt.name == "empty" && !strings.Contains(logs.String(), `"level":"WARN"`) {
				t.Fatal("empty feed must be a warning")
			}
		})
	}
}

func TestEmptyGroupAndTotals(t *testing.T) {
	logs := captureScrapeLogs(t)

	stats := processBlogs(context.Background(), nil, nil, config.ScraperConfig{}, nil, "empty")
	if stats.EmptyGroups != 1 || stats.PostsSent != 0 {
		t.Fatalf("bad empty stats: %+v", stats)
	}

	var total scrapeStats
	total.add(stats)
	total.add(
		scrapeStats{
			PostsFound:         2,
			PostsSent:          1,
			RecipientsAccepted: 3,
			RecipientsPartial:  1,
			RecipientsFailed:   2,
			Errors:             4,
		},
	)

	if total.EmptyGroups != 1 || total.PostsSent != 1 || total.RecipientsAccepted != 3 || total.RecipientsPartial != 1 ||
		total.RecipientsFailed != 2 ||
		total.Errors != 4 {
		t.Fatalf("bad totals: %+v", total)
	}

	if !strings.Contains(logs.String(), `"empty_groups":1`) {
		t.Fatal("missing empty group counter")
	}
}
