package observability

import (
	"context"
	"testing"
)

func TestWithRequestID(t *testing.T) {
	ctx := WithRequestID(context.Background())

	id := RequestID(ctx)
	if len(id) != 8 {
		t.Errorf("RequestID length = %d, want 8", len(id))
	}

	if id == "" {
		t.Error("RequestID is empty, want non-empty")
	}
}

func TestWithRunID(t *testing.T) {
	ctx := WithRunID(context.Background(), "scrape")

	id := RequestID(ctx)
	if len(id) != 8 {
		t.Errorf("RequestID length = %d, want 8", len(id))
	}

	if id == "" {
		t.Error("RequestID is empty, want non-empty")
	}
}

func TestRequestID_EmptyContext(t *testing.T) {
	id := RequestID(context.Background())
	if id != "" {
		t.Errorf("RequestID = %q, want empty string", id)
	}
}
