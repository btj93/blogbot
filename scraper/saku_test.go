package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSakuURLFormat(t *testing.T) {
	href := "/s/s46/diary/blog/detail/12345?ima=0450&cd=blog"
	want := "https://sakurazaka46.com/s/s46/diary/blog/detail/12345?ima=0000&cd=blog"

	path, _, _ := strings.Cut(href, "?")

	got := "https://sakurazaka46.com" + path + "?ima=0000&cd=blog"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSakuScraper_Scrape(t *testing.T) {
	listHTML, err := os.ReadFile("testdata/saku_blog_list.html")
	if err != nil {
		t.Fatalf("reading list fixture: %v", err)
	}

	detailHTML, err := os.ReadFile("testdata/saku_blog_detail.html")
	if err != nil {
		t.Fatalf("reading detail fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "list") {
			if _, err := w.Write(listHTML); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if _, err := w.Write(detailHTML); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}))
	defer ts.Close()

	s := &SakuScraper{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		UserAgent:  "test",
	}

	blogs, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error: %v", err)
	}

	if len(blogs) == 0 {
		t.Fatal("expected blogs, got 0")
	}

	for i, b := range blogs {
		if b.Name == "" {
			t.Errorf("blog[%d].Name is empty", i)
		}

		if b.URL == "" {
			t.Errorf("blog[%d].URL is empty", i)
		}
	}
}

func TestSakuScraper_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := &SakuScraper{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		UserAgent:  "test",
	}

	_, err := s.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}
