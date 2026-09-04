package scraper

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/btj93/blogbot/model"
)

type Scraper interface {
	Scrape(ctx context.Context) ([]model.Blog, error)
}

var defaultHeaders = http.Header{
	"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
	"Accept-Encoding":           {"gzip, deflate"},
	"Accept-Language":           {"en-GB,en-US;q=0.9,en;q=0.8"},
	"Dnt":                       {"1"},
	"Upgrade-Insecure-Requests": {"1"},
}

func newRequest(ctx context.Context, url, userAgent string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	maps.Copy(req.Header, defaultHeaders)

	req.Header.Set("User-Agent", userAgent)

	return req, nil
}

func fetchBody(ctx context.Context, url, userAgent string, client *http.Client) ([]byte, error) {
	req, err := newRequest(ctx, url, userAgent)
	if err != nil {
		return nil, err
	}

	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

// DownloadImages downloads images concurrently with bounded parallelism.
// Filters out nil/failed downloads. Returns flat slice of valid image bytes.
func DownloadImages(ctx context.Context, urls []string, userAgent string, client *http.Client) ([][]byte, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	sem := semaphore.NewWeighted(10)

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		images [][]byte
	)

	for _, u := range urls {
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, err
		}

		wg.Go(func() {
			defer sem.Release(1)

			data := downloadOneImage(ctx, u, userAgent, client)
			if data != nil {
				mu.Lock()

				images = append(images, data)
				mu.Unlock()
			}
		})
	}

	wg.Wait()

	return images, nil
}

func downloadOneImage(ctx context.Context, url, userAgent string, client *http.Client) []byte {
	if strings.Contains(url, "null") {
		return nil
	}

	if client == nil {
		client = http.DefaultClient
	}

	req, err := newRequest(ctx, url, userAgent)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// Handle dcimg.awalker.jp cookie redirect
	if strings.Contains(url, "dcimg.awalker.jp") {
		redirectURL := strings.Replace(url, "/v/", "/i/", 1)

		req2, err := newRequest(ctx, redirectURL, userAgent)
		if err != nil {
			return nil
		}

		for _, c := range resp.Cookies() {
			req2.AddCookie(c)
		}

		resp2, err := client.Do(req2)
		if err != nil {
			return nil
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			return nil
		}

		data, _ := io.ReadAll(resp2.Body)

		return data
	}

	data, _ := io.ReadAll(resp.Body)

	return data
}

// BatchImagesGrouped splits images into groups of maxSize for Telegram media group limit.
func BatchImagesGrouped(images [][]byte, maxSize int) [][][]byte {
	if len(images) == 0 {
		return nil
	}

	var batches [][][]byte

	for i := 0; i < len(images); i += maxSize {
		end := min(i+maxSize, len(images))

		batches = append(batches, images[i:end])
	}

	return batches
}
