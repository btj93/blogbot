package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDownloadImagesPreservesInputOrder(t *testing.T) {
	lastServed := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/first":
			select {
			case <-lastServed:
			case <-r.Context().Done():
				return
			}

			time.Sleep(50 * time.Millisecond)

			_, _ = fmt.Fprint(w, "first")
		case "/failed":
			w.WriteHeader(http.StatusNotFound)
		case "/last":
			_, _ = fmt.Fprint(w, "last")

			close(lastServed)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	images, err := DownloadImages(
		ctx,
		[]string{server.URL + "/first", server.URL + "/failed", server.URL + "/last"},
		"test",
		server.Client(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != 2 || string(images[0]) != "first" || string(images[1]) != "last" {
		t.Fatalf("images = %q, want [first last] despite reversed completion and a failed download", images)
	}
}
