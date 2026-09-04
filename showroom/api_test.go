package showroom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchNextLive(t *testing.T) {
	data, err := os.ReadFile("testdata/showroom_nextlive.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	origBase := ShowroomBaseURL
	origClient := ShowroomClient
	ShowroomBaseURL = ts.URL
	ShowroomClient = ts.Client()

	t.Cleanup(func() {
		ShowroomBaseURL = origBase
		ShowroomClient = origClient
	})

	resp, err := FetchNextLive(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Epoch == nil {
		t.Fatal("expected Epoch != nil")
	}

	if resp.Text == "" {
		t.Fatal("expected Text != \"\"")
	}
}

func TestFetchNextLive_EmptyRoomID(t *testing.T) {
	resp, err := FetchNextLive(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "TBD" {
		t.Fatalf("expected Text=\"TBD\", got %q", resp.Text)
	}
}

func TestFetchNextLive_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	origBase := ShowroomBaseURL
	origClient := ShowroomClient
	ShowroomBaseURL = ts.URL
	ShowroomClient = ts.Client()

	t.Cleanup(func() {
		ShowroomBaseURL = origBase
		ShowroomClient = origClient
	})

	_, err := FetchNextLive(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchStreamingURL(t *testing.T) {
	data, err := os.ReadFile("testdata/showroom_streamingurl.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer ts.Close()

	origBase := ShowroomBaseURL
	origClient := ShowroomClient
	ShowroomBaseURL = ts.URL
	ShowroomClient = ts.Client()

	t.Cleanup(func() {
		ShowroomBaseURL = origBase
		ShowroomClient = origClient
	})

	url, err := FetchStreamingURL(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should pick highest quality HLS, not the webrtc with quality 1000
	expected := "https://example.com/live/main_ss.m3u8"
	if url != expected {
		t.Fatalf("expected %q, got %q", expected, url)
	}
}

func TestFetchStreamingURL_NoURLs(t *testing.T) {
	serveJSON(t, `{"streaming_url_list":[]}`)

	_, err := FetchStreamingURL(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error for empty streaming URL list")
	}
}

func TestFetchStreamingURL_WebRTCOnly(t *testing.T) {
	// When only webrtc entries exist, should return error (no HLS or https fallback)
	serveJSON(t, `{"streaming_url_list":[
		{"url":"webrtc://example.com/main_ss","quality":1000,"type":"webrtc"}
	]}`)

	_, err := FetchStreamingURL(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error when only webrtc URLs available")
	}
}

func TestFetchStreamingURL_HLSAllFallback(t *testing.T) {
	// No "hls" type, but "hls_all" has https:// URL — should use it as fallback
	serveJSON(t, `{"streaming_url_list":[
		{"url":"webrtc://example.com/main_ss","quality":1000,"type":"webrtc"},
		{"url":"https://example.com/main_abr.m3u8","quality":0,"type":"hls_all"}
	]}`)

	url, err := FetchStreamingURL(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "https://example.com/main_abr.m3u8" {
		t.Fatalf("expected hls_all fallback, got %q", url)
	}
}

func serveJSON(t *testing.T, body string) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))

	origBase := ShowroomBaseURL
	origClient := ShowroomClient
	ShowroomBaseURL = ts.URL
	ShowroomClient = ts.Client()

	t.Cleanup(func() {
		ts.Close()

		ShowroomBaseURL = origBase
		ShowroomClient = origClient
	})
}
