package scraper

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeGzip(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	w.Header().Set("Content-Encoding", "gzip")

	writer := gzip.NewWriter(w)
	if _, err := writer.Write(data); err != nil {
		t.Error(err)
	}

	if err := writer.Close(); err != nil {
		t.Error(err)
	}
}

func TestRequestLeavesCompressionToTransport(t *testing.T) {
	req, err := newRequest(context.Background(), "https://example.com", "test")
	if err != nil {
		t.Fatal(err)
	}

	if got := req.Header.Get("Accept-Encoding"); got != "" {
		t.Fatalf("explicit Accept-Encoding = %q, want transport-managed compression", got)
	}
}

func TestFetchBodyGzip(t *testing.T) {
	want := []byte(`{"data":[{"title":"compressed blog"}]}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept-Encoding"); got != "gzip" {
			t.Errorf("Accept-Encoding = %q, want gzip", got)
		}

		writeGzip(t, w, want)
	}))
	defer server.Close()

	got, err := fetchBody(context.Background(), server.URL, "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("fetchBody = %q, want decoded %q", got, want)
	}
}

func TestDownloadImageGzip(t *testing.T) {
	want := []byte("decoded image bytes")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeGzip(t, w, want)
	}))
	defer server.Close()

	got := downloadOneImage(context.Background(), server.URL, "test", server.Client())
	if !bytes.Equal(got, want) {
		t.Fatalf("image = %q, want decoded %q", got, want)
	}
}
