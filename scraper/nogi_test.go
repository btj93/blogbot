package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNogiScraperJSONPStripping(t *testing.T) {
	response := `res({"data":[{"name":"遠藤 さくら","title":"テストブログ","link":"https://nogizaka46.com/s/n46/diary/detail/12345?ima=0","text":"<p>Hello</p><img src=\"/s/n46/img/test.jpg\">"}]});`

	content := response
	if len(content) > 4 && content[:4] == "res(" {
		content = content[4 : len(content)-2]
	}

	if content[0] != '{' {
		t.Errorf("JSONP not stripped: %q", content[:10])
	}
}

func TestNogiScraper_Scrape(t *testing.T) {
	data, err := os.ReadFile("testdata/nogi_blog_list.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	s := &NogiScraper{
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

func TestNogiScraper_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := &NogiScraper{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		UserAgent:  "test",
	}

	_, err := s.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestNogiScraper_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("not json")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	s := &NogiScraper{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		UserAgent:  "test",
	}

	_, err := s.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestNogiScraper_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	s := &NogiScraper{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
		UserAgent:  "test",
	}

	blogs, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape() error: %v", err)
	}

	if len(blogs) != 0 {
		t.Errorf("expected 0 blogs, got %d", len(blogs))
	}
}
