package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/btj93/blogbot/observability"
)

type memberImage struct {
	Name string `json:"name"`
	Img  string `json:"img"`
}

type nogiAPIData struct {
	Name string `json:"name"`
	Img  string `json:"img"`
}

type nogiAPIResponse struct {
	Data []nogiAPIData `json:"data"`
}

// memberImagesResponse is map[groupName][]memberImage.
type memberImagesResponse map[string][]memberImage

// memberImagesCache caches the scraped member images for 1 hour.
var memberImagesCache struct {
	mu        sync.Mutex
	data      *memberImagesResponse
	expiresAt time.Time
}

// HandleMemberImages serves GET /tg/blogbot/api/v1/member-images.
// No auth required — this is public data from official sites.
func HandleMemberImages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)

		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})

		return
	}

	ctx := observability.WithRequestID(r.Context())

	// Check cache.
	memberImagesCache.mu.Lock()
	if memberImagesCache.data != nil && time.Now().Before(memberImagesCache.expiresAt) {
		data := memberImagesCache.data
		memberImagesCache.mu.Unlock()
		writeJSON(w, http.StatusOK, data)

		return
	}
	memberImagesCache.mu.Unlock()

	// Fetch all three in parallel.
	var (
		nogi []memberImage
		saku []memberImage
		hina []memberImage
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	wg.Go(func() {
		result, err := fetchNogizakaImages(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch nogizaka images", slog.Any("error", err))

			return
		}

		mu.Lock()
		nogi = result
		mu.Unlock()
	})

	wg.Go(func() {
		result, err := fetchSakurazakaImages(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch sakurazaka images", slog.Any("error", err))

			return
		}

		mu.Lock()
		saku = result
		mu.Unlock()
	})

	wg.Go(func() {
		result, err := fetchHinatazakaImages(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch hinatazaka images", slog.Any("error", err))

			return
		}

		mu.Lock()
		hina = result
		mu.Unlock()
	})

	wg.Wait()

	resp := &memberImagesResponse{
		"乃木坂46": nogi,
		"櫻坂46":  saku,
		"日向坂46": hina,
	}

	// Cache for 1 hour.
	memberImagesCache.mu.Lock()
	memberImagesCache.data = resp
	memberImagesCache.expiresAt = time.Now().Add(time.Hour)
	memberImagesCache.mu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}

	return client.Do(req)
}

// fetchNogizakaImages fetches from the Nogizaka46 JSONP API.
func fetchNogizakaImages(ctx context.Context) ([]memberImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://nogizaka46.com/s/n46/api/list/member", nil)
	if err != nil {
		return nil, fmt.Errorf("nogizaka request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nogizaka fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nogizaka read: %w", err)
	}

	text := string(body)
	if !strings.HasPrefix(text, "res(") {
		return nil, errors.New("nogizaka: unexpected response format")
	}

	// Strip JSONP wrapper: res({...});
	jsonStr := text[4 : len(text)-2]

	var parsed nogiAPIResponse
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("nogizaka parse: %w", err)
	}

	result := make([]memberImage, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		result = append(result, memberImage(m))
	}

	return result, nil
}

// fetchSakurazakaImages scrapes the Sakurazaka46 member page.
func fetchSakurazakaImages(ctx context.Context) ([]memberImage, error) {
	resp, err := httpGet(ctx, "https://sakurazaka46.com/s/s46/search/artist?ima=0000")
	if err != nil {
		return nil, fmt.Errorf("sakurazaka fetch: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sakurazaka parse: %w", err)
	}

	var result []memberImage

	doc.Find("p.name").Each(func(_ int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Text())
		if name == "" {
			return
		}

		// The image is in a sibling <img> element before the <p class="name">.
		img := s.Parent().Find("img").First()

		imgSrc, _ := img.Attr("src")
		if imgSrc != "" && !strings.HasPrefix(imgSrc, "http") {
			imgSrc = "https://sakurazaka46.com" + imgSrc
		}

		result = append(result, memberImage{
			Name: strings.ReplaceAll(name, " ", ""),
			Img:  imgSrc,
		})
	})

	return result, nil
}

// fetchHinatazakaImages scrapes the Hinatazaka46 member page.
func fetchHinatazakaImages(ctx context.Context) ([]memberImage, error) {
	resp, err := httpGet(ctx, "https://www.hinatazaka46.com/s/official/search/artist?ima=0000")
	if err != nil {
		return nil, fmt.Errorf("hinatazaka fetch: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hinatazaka parse: %w", err)
	}

	var result []memberImage

	doc.Find(".c-member__name").Each(func(_ int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Text())
		if name == "" {
			return
		}

		// The image is in the preceding .c-member__thumb sibling.
		thumb := s.Parent().Find(".c-member__thumb img").First()

		imgSrc, _ := thumb.Attr("src")

		result = append(result, memberImage{
			Name: strings.ReplaceAll(name, " ", ""),
			Img:  imgSrc,
		})
	})

	return result, nil
}
