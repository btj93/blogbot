package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/btj93/blogbot/config"
	"github.com/btj93/blogbot/model"
)

func TestProcessBlogsCompletesEachPostBeforeNext(t *testing.T) {
	logs := captureScrapeLogs(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "photo")
	}))
	defer server.Close()

	oldest := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	blogs := []model.Blog{
		{
			Name:        "member",
			Title:       "old",
			URL:         "https://example.com/old",
			ImageURLs:   []string{server.URL + "/old"},
			PublishedAt: oldest,
		},
		{
			Name:        "member",
			Title:       "new",
			URL:         "https://example.com/new",
			ImageURLs:   []string{server.URL + "/new"},
			PublishedAt: oldest.Add(24 * time.Hour),
		},
	}
	db := &deliveryDB{chats: []string{"42", "43"}}
	tg := &deliverySender{}

	stats := processBlogs(context.Background(), db, tg, config.ScraperConfig{}, blogs, "test")
	if stats.PostsSent != 2 || stats.RecipientsAccepted != 4 {
		t.Fatalf("unexpected delivery stats: %+v", stats)
	}

	want := make([]string, 0, 2*len(db.chats)*len(blogs))

	for _, blog := range blogs {
		for _, chat := range db.chats {
			want = append(want,
				fmt.Sprintf("%s:text:#member 「%s」\n\n%s", chat, blog.Title, blog.URL),
				fmt.Sprintf("%s:media:[photo]", chat),
			)
		}
	}

	if !slices.Equal(tg.events, want) {
		t.Fatalf("delivery events = %q, want %q", tg.events, want)
	}

	if !strings.Contains(logs.String(), `"published_at":"2026-11-01T00:00:00Z"`) {
		t.Fatalf("publication time missing from delivery logs: %s", logs.String())
	}
}
