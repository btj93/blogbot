package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHinaScraper_Scrape(t *testing.T) {
	data, err := os.ReadFile("testdata/hina_blog_list.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	s := &HinaScraper{
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

func TestHinaScraper_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := &HinaScraper{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		UserAgent:  "test",
	}

	_, err := s.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}
