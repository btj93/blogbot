package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 1x1 pixel valid PNG
var pngPixel = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
	0x44, 0xAE, 0x42, 0x60, 0x82,
}

func TestDownloadImages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngPixel)
	}))
	defer ts.Close()

	urls := make([]string, 0, 15)
	for i := range 15 {
		urls = append(urls, fmt.Sprintf("%s/image%d.png", ts.URL, i))
	}

	images, err := DownloadImages(context.Background(), urls, "test", ts.Client())
	if err != nil {
		t.Fatalf("DownloadImages() error: %v", err)
	}

	if len(images) != 15 {
		t.Errorf("got %d images, want 15", len(images))
	}
}

func TestDownloadImages_Empty(t *testing.T) {
	images, err := DownloadImages(context.Background(), nil, "test", nil)
	if err != nil {
		t.Fatalf("DownloadImages() error: %v", err)
	}

	if images != nil {
		t.Errorf("expected nil, got %d images", len(images))
	}
}

func TestDownloadImages_ContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(pngPixel)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := DownloadImages(ctx, []string{ts.URL + "/image.png"}, "test", ts.Client())
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
