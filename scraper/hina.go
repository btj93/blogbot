package scraper

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/btj93/blogbot/model"
)

type HinaScraper struct {
	UserAgent  string
	BaseURL    string       // default: "https://www.hinatazaka46.com"
	HTTPClient *http.Client // default: http.DefaultClient
}

func (s *HinaScraper) Scrape(ctx context.Context) ([]model.Blog, error) {
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = "https://www.hinatazaka46.com"
	}

	body, err := fetchBody(
		ctx,
		baseURL+"/s/official/diary/member/list?ima=0000&page=0&cd=member",
		s.UserAgent,
		s.HTTPClient,
	)
	if err != nil {
		return nil, fmt.Errorf("hina scrape: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("hina parse: %w", err)
	}

	var (
		result   []model.Blog
		parseErr error
	)

	jst := time.FixedZone("JST", 9*60*60)

	doc.Find("div.l-maincontents--blog div.p-button__blog_detail").Each(func(_ int, btnDiv *goquery.Selection) {
		if parseErr != nil {
			return
		}

		parent := btnDiv.Parent()
		articleHead := parent.Closest("div.p-blog-article")

		title := strings.TrimSpace(articleHead.Find("div.c-blog-article__title").First().Text())
		name := strings.TrimSpace(strings.ReplaceAll(articleHead.Find("div.c-blog-article__name").First().Text(), " ", ""))

		href, exists := parent.Find("a.c-button-blog-detail").First().Attr("href")
		if !exists {
			return
		}

		url := baseURL + href
		date := strings.Join(strings.Fields(articleHead.Find("div.c-blog-article__date").First().Text()), " ")

		publishedAt, err := time.ParseInLocation("2006.1.2 15:04", date, jst)
		if err != nil {
			parseErr = fmt.Errorf("hina publication date for %s: %w", url, err)

			return
		}

		var imgURLs []string

		parent.Find("img[src]").Each(func(_ int, sel *goquery.Selection) {
			src, exists := sel.Attr("src")
			if !exists || src == "" || strings.Contains(src, "blob:") {
				return
			}

			imgURLs = append(imgURLs, src)
		})

		result = append(result, model.Blog{
			Title:       title,
			Name:        name,
			URL:         url,
			ImageURLs:   imgURLs,
			PublishedAt: publishedAt,
		})
	})

	if parseErr != nil {
		return nil, parseErr
	}

	// Keep the site's oldest-first order for posts sharing the same publication minute.
	slices.Reverse(result)
	slices.SortStableFunc(result, func(a, b model.Blog) int {
		return a.PublishedAt.Compare(b.PublishedAt)
	})

	return result, nil
}
