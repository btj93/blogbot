package store

import (
	"context"
	"testing"
)

func TestScrapeLockSerializesWorkers(t *testing.T) {
	first := newTestStore(t)

	second, err := Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = second.Close() })

	ctx := context.Background()

	release, acquired, err := first.TryScrapeLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("first lock: acquired=%v error=%v", acquired, err)
	}

	t.Cleanup(func() { _ = release() })

	otherRelease, acquired, err := second.TryScrapeLock(ctx)
	if err != nil || acquired || otherRelease != nil {
		t.Fatalf("overlapping worker: acquired=%v release_is_nil=%v error=%v", acquired, otherRelease == nil, err)
	}

	if err := release(); err != nil {
		t.Fatal(err)
	}

	otherRelease, acquired, err = second.TryScrapeLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("lock after release: acquired=%v error=%v", acquired, err)
	}

	t.Cleanup(func() { _ = otherRelease() })
}

func TestScrapeLockConnectionLoss(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	release, acquired, err := s.TryScrapeLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("first lock: acquired=%v error=%v", acquired, err)
	}

	t.Cleanup(func() { _ = release() })

	var terminated bool

	err = s.db.QueryRowContext(ctx, "SELECT pg_terminate_backend(pid) FROM pg_locks WHERE locktype='advisory' AND granted AND classid::bigint=$1 AND objid::bigint=$2 AND objsubid=1", scrapeLockID>>32, scrapeLockID&0xffffffff).
		Scan(&terminated)
	if err != nil || !terminated {
		t.Fatalf("terminate lock session: terminated=%v error=%v", terminated, err)
	}

	if err := release(); err == nil {
		t.Fatal("expected release error after session loss")
	}

	nextRelease, acquired, err := s.TryScrapeLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("lock after session loss: acquired=%v error=%v", acquired, err)
	}

	t.Cleanup(func() { _ = nextRelease() })
}

func TestScrapeLockFailsClosed(t *testing.T) {
	s := newTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	release, acquired, err := s.TryScrapeLock(context.Background())
	if err == nil || acquired || release != nil {
		t.Fatalf("closed DB: acquired=%v release_is_nil=%v error=%v", acquired, release == nil, err)
	}
}
