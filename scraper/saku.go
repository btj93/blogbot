package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/btj93/blogbot/model"
)

type SakuScraper struct {
	UserAgent  string
	BaseURL    string       // default: "https://sakurazaka46.com"
	HTTPClient *http.Client // default: http.DefaultClient
}

func (s *SakuScraper) Scrape(ctx context.Context) ([]model.Blog, error) {
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = "https://sakurazaka46.com"
	}

	body, err := fetchBody(ctx, baseURL+"/s/s46/diary/blog/list?ima=0450&page=0&cd=blog", s.UserAgent, s.HTTPClient)
	if err != nil {
		return nil, fmt.Errorf("saku scrape list: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("saku parse list: %w", err)
	}

	var result []model.Blog

	doc.Find("ul.com-blog-part li.box").Each(func(_ int, item *goquery.Selection) {
		title := strings.TrimSpace(item.Find("h3.title").Text())
		name := strings.TrimSpace(strings.ReplaceAll(item.Find("p.name").Text(), " ", ""))

		href, exists := item.Find("a[href]").Attr("href")
		if !exists {
			return
		}

		url := baseURL + strings.SplitN(href, "?", 2)[0] + "?ima=0000&cd=blog"

		imgURLs := s.fetchDetailImages(ctx, baseURL, url)

		result = append(result, model.Blog{
			Title:     title,
			Name:      name,
			URL:       url,
			ImageURLs: imgURLs,
		})
	})

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

func (s *SakuScraper) fetchDetailImages(ctx context.Context, baseURL, url string) []string {
	body, err := fetchBody(ctx, url, s.UserAgent, s.HTTPClient)
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var imgs []string

	doc.Find("div.box-article img[src]").Each(func(_ int, sel *goquery.Selection) {
		src, exists := sel.Attr("src")
		if !exists || src == "" || strings.Contains(src, "null") {
			return
		}

		if strings.HasPrefix(src, "http") {
			imgs = append(imgs, src)
		} else {
			imgs = append(imgs, baseURL+src)
		}
	})

	return imgs
}
