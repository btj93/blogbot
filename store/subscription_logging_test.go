package store

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/btj93/blogbot/observability"
)

func TestSubscriptionMutationLogs(t *testing.T) {
	s := newTestStore(t)
	ctx := observability.WithAttrs(context.Background(), slog.Int64("user_id", 99))

	group, err := s.GetOrCreateGroup(ctx, "audit test")
	if err != nil {
		t.Fatal(err)
	}

	generation := 1

	one, err := s.GetOrCreateMember(ctx, group.ID, "one", &generation, false)
	if err != nil {
		t.Fatal(err)
	}

	two, err := s.GetOrCreateMember(ctx, group.ID, "two", &generation, false)
	if err != nil {
		t.Fatal(err)
	}

	ungrouped, err := s.GetOrCreateMember(ctx, group.ID, "ungrouped", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	disabled, err := s.GetOrCreateMember(ctx, group.ID, "disabled", &generation, true)
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(&logs, nil), nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	for range 2 {
		if _, err := s.AddSubscription(ctx, one.ID, "42"); err != nil {
			t.Fatal(err)
		}
	}

	for range 2 {
		if _, err := s.RemoveSubscription(ctx, one.ID, "42"); err != nil {
			t.Fatal(err)
		}
	}

	for _, gen := range []*int{&generation, nil} {
		for range 2 {
			if err := s.AddSubscriptionForAllInGeneration(ctx, group.ID, gen, "42"); err != nil {
				t.Fatal(err)
			}
		}

		for range 2 {
			if err := s.RemoveSubscriptionForAllInGeneration(ctx, group.ID, gen, "42"); err != nil {
				t.Fatal(err)
			}
		}
	}

	if count := strings.Count(logs.String(), `"msg":"subscription changed"`); count != 8 {
		t.Fatalf("expected 8 actual mutation events, got %d: %s", count, logs.String())
	}

	for _, memberID := range []int64{one.ID, two.ID, ungrouped.ID} {
		if !strings.Contains(logs.String(), `"member_id":`+strconv.FormatInt(memberID, 10)) {
			t.Errorf("missing member %d", memberID)
		}
	}

	if strings.Contains(logs.String(), `"member_id":`+strconv.FormatInt(disabled.ID, 10)+",") {
		t.Fatal("disabled member incorrectly included in bulk mutation")
	}

	if !strings.Contains(logs.String(), `"user_id":99`) || !strings.Contains(logs.String(), `"chat_id":"42"`) {
		t.Fatalf("missing actor or destination: %s", logs.String())
	}
}
