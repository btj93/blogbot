package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func hinaArticle(title, date string) string {
	return fmt.Sprintf(`<div class="p-blog-article">
<div class="c-blog-article__title">%s</div>
<div class="c-blog-article__name">鶴崎 仁香</div>
<div class="c-blog-article__date">%s</div>
<div class="p-button__blog_detail"><a class="c-button-blog-detail" href="/%s"></a></div>
</div>`, title, date, title)
}

func TestHinaScraperPublicationOrder(t *testing.T) {
	var page strings.Builder

	_, _ = page.WriteString(`<div class="l-maincontents--blog">`)

	for _, article := range []struct{ title, date string }{
		{"newer-tie", "2026.11.3 21:42"},
		{"oldest", "2026.11.1 15:02"},
		{"older-tie", "2026.11.3 21:42"},
		{"newest", "2026.11.4 18:18"},
	} {
		_, _ = page.WriteString(hinaArticle(article.title, article.date))
	}

	_, _ = page.WriteString("</div>")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, page.String())
	}))
	defer server.Close()

	s := &HinaScraper{BaseURL: server.URL, HTTPClient: server.Client()}

	blogs, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	titles := make([]string, 0, len(blogs))
	for _, blog := range blogs {
		titles = append(titles, blog.Title)
	}

	if want := []string{"oldest", "older-tie", "newer-tie", "newest"}; !slices.Equal(titles, want) {
		t.Fatalf("publication order = %v, want %v", titles, want)
	}

	if want := time.Date(2026, time.November, 1, 6, 2, 0, 0, time.UTC); !blogs[0].PublishedAt.Equal(want) {
		t.Fatalf("published_at = %v, want JST interpreted as %v", blogs[0].PublishedAt, want)
	}
}

func TestHinaScraperRejectsUnknownPublicationOrder(t *testing.T) {
	for _, tt := range []struct{ name, date string }{{"missing", ""}, {"invalid", "not a date"}} {
		t.Run(tt.name, func(t *testing.T) {
			page := `<div class="l-maincontents--blog">` +
				hinaArticle("valid", "2026.11.1 15:02") + hinaArticle("invalid", tt.date) + "</div>"

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, page)
			}))
			defer server.Close()

			s := &HinaScraper{BaseURL: server.URL, HTTPClient: server.Client()}

			blogs, err := s.Scrape(context.Background())
			if err == nil || len(blogs) != 0 {
				t.Fatalf("Scrape() = %v, %v; want no posts and an error rather than guessed order", blogs, err)
			}
		})
	}
}
