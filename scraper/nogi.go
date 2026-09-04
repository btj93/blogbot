package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/btj93/blogbot/model"
)

type NogiScraper struct {
	UserAgent  string
	BaseURL    string       // default: "https://nogizaka46.com"
	HTTPClient *http.Client // default: http.DefaultClient
}

type nogiAPIResponse struct {
	Data []nogiEntry `json:"data"`
}

type nogiEntry struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Link  string `json:"link"`
	Text  string `json:"text"`
}

func (s *NogiScraper) Scrape(ctx context.Context) ([]model.Blog, error) {
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = "https://nogizaka46.com"
	}

	body, err := fetchBody(ctx, baseURL+"/s/n46/api/list/blog", s.UserAgent, s.HTTPClient)
	if err != nil {
		return nil, fmt.Errorf("nogi scrape: %w", err)
	}

	content := string(body)
	if strings.HasPrefix(content, "res(") {
		content = content[4 : len(content)-2]
	}

	var resp nogiAPIResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("nogi parse: %w", err)
	}

	var result []model.Blog

	limit := min(len(resp.Data), 20)

	for _, entry := range resp.Data[:limit] {
		if entry.Text == "" {
			continue
		}

		name := strings.TrimSpace(strings.ReplaceAll(entry.Name, " ", ""))
		url, _, _ := strings.Cut(entry.Link, "?")

		var imgURLs []string

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(entry.Text))
		if err == nil {
			doc.Find("img[src]").Each(func(_ int, sel *goquery.Selection) {
				src, exists := sel.Attr("src")
				if !exists || src == "" || strings.Contains(src, "null") {
					return
				}

				if strings.HasPrefix(src, "http") {
					imgURLs = append(imgURLs, src)
				} else {
					imgURLs = append(imgURLs, baseURL+src)
				}
			})
		}

		result = append(result, model.Blog{
			Title:     entry.Title,
			Name:      name,
			URL:       url,
			ImageURLs: imgURLs,
		})
	}

	// Reverse to chronological order (oldest first)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}
