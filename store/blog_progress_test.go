package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestClaimBlogConcurrentWorkers(t *testing.T) {
	s := newTestStore(t)
	start := make(chan struct{})

	var (
		workers sync.WaitGroup
		winners atomic.Int32
	)

	for range 10 {
		workers.Go(func() {
			<-start

			claimed, err := s.ClaimBlog(context.Background(), "https://example.com/concurrent-post")
			if err != nil {
				t.Error(err)
				return
			}

			if claimed {
				winners.Add(1)
			}
		})
	}

	close(start)
	workers.Wait()

	if got := winners.Load(); got != 1 {
		t.Fatalf("successful claims = %d, want exactly one across concurrent workers", got)
	}
}
