package showroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
)

var (
	// ShowroomBaseURL is the base URL for the Showroom API. Override in tests.
	ShowroomBaseURL = "https://www.showroom-live.com"
	// ShowroomClient is the HTTP client used for Showroom API requests. Override in tests.
	ShowroomClient = http.DefaultClient
	apiHeaders     = http.Header{
		"User-Agent": {
			"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.76 Safari/537.36",
		},
		"Upgrade-Insecure-Requests": {"1"},
		"DNT":                       {"1"},
		"Accept":                    {"application/json"},
		"Accept-Language":           {"en-US,en;q=0.5"},
	}
)

type NextLiveResponse struct {
	Epoch *int64 `json:"epoch"`
	Text  string `json:"text"`
}

type RoomStatus struct {
	BroadcastKey  string `json:"broadcast_key"`
	BroadcastHost string `json:"broadcast_host"`
	BroadcastPort int    `json:"broadcast_port"`
}

type StreamingURL struct {
	StreamingURLList []StreamingURLEntry `json:"streaming_url_list"`
}

type StreamingURLEntry struct {
	URL     string `json:"url"`
	Quality int    `json:"quality"`
	Type    string `json:"type"`
}

func FetchNextLive(ctx context.Context, roomID string) (*NextLiveResponse, error) {
	if roomID == "" {
		return &NextLiveResponse{Text: "TBD"}, nil
	}

	reqURL := ShowroomBaseURL + "/api/room/next_live?" + url.Values{"room_id": {roomID}}.Encode()

	body, err := apiGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var resp NextLiveResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing next_live: %w", err)
	}

	return &resp, nil
}

func FetchRoomStatus(ctx context.Context, roomURL string) (*RoomStatus, error) {
	urlKey := roomURL[strings.LastIndex(roomURL, "/")+1:]
	reqURL := ShowroomBaseURL + "/api/room/status?" + url.Values{"room_url_key": {urlKey}}.Encode()

	body, err := apiGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var resp RoomStatus
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing room status: %w", err)
	}

	return &resp, nil
}

func FetchStreamingURL(ctx context.Context, roomID string) (string, error) {
	reqURL := ShowroomBaseURL + "/api/live/streaming_url?" + url.Values{"room_id": {roomID}}.Encode()

	body, err := apiGet(ctx, reqURL)
	if err != nil {
		return "", err
	}

	var resp StreamingURL
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parsing streaming url: %w", err)
	}

	if len(resp.StreamingURLList) == 0 {
		return "", errors.New("no streaming URLs")
	}

	// Prefer HLS streams — WebRTC URLs are not usable in Telegram messages.
	var hlsList []StreamingURLEntry

	for _, entry := range resp.StreamingURLList {
		if entry.Type == "hls" {
			hlsList = append(hlsList, entry)
		}
	}

	candidates := hlsList
	if len(candidates) == 0 {
		// Fallback: any entry with an https:// URL
		for _, entry := range resp.StreamingURLList {
			if strings.HasPrefix(entry.URL, "https://") {
				candidates = append(candidates, entry)
			}
		}
	}

	if len(candidates) == 0 {
		return "", errors.New("no HLS streaming URLs")
	}

	best := candidates[0]
	for _, entry := range candidates[1:] {
		if entry.Quality > best.Quality {
			best = entry
		}
	}

	return best.URL, nil
}

func apiGet(ctx context.Context, reqURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	maps.Copy(req.Header, apiHeaders)

	resp, err := ShowroomClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, reqURL)
	}

	return io.ReadAll(resp.Body)
}
