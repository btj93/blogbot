package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btj93/blogbot/model"
)

// ---------------------------------------------------------------------------
// HTTP layer tests
// ---------------------------------------------------------------------------

func TestHandleMembers_MethodNotAllowed(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/members", nil)
	w := httptest.NewRecorder()
	h.HandleMembers(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/members: got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleMembers_MissingInitData(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/members", nil)
	w := httptest.NewRecorder()
	h.HandleMembers(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/members without header: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleMembers_InvalidInitData(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/members", nil)
	req.Header.Set("X-Telegram-Init-Data", "garbage-data-here")

	w := httptest.NewRecorder()
	h.HandleMembers(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/members with garbage header: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleSubscriptions_MethodNotAllowed(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/subscriptions", nil)
	w := httptest.NewRecorder()
	h.HandleSubscriptions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/subscriptions: got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleSubscriptions_MissingInitData(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/subscriptions",
		strings.NewReader(`{"changes":[]}`),
	)
	w := httptest.NewRecorder()
	h.HandleSubscriptions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/subscriptions without header: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestCORS_Preflight(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	// Test OPTIONS on HandleMembers.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/members", nil)
	w := httptest.NewRecorder()
	h.HandleMembers(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS /api/members: got status %d, want %d", w.Code, http.StatusNoContent)
	}

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS Allow-Origin = %q, want *", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-Telegram-Init-Data") {
		t.Errorf("CORS Allow-Headers = %q, want to contain X-Telegram-Init-Data", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "GET") {
		t.Errorf("CORS Allow-Methods = %q, want to contain GET", got)
	}

	// Test OPTIONS on HandleSubscriptions.
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/subscriptions", nil)
	w2 := httptest.NewRecorder()
	h.HandleSubscriptions(w2, req2)

	if w2.Code != http.StatusNoContent {
		t.Errorf("OPTIONS /api/subscriptions: got status %d, want %d", w2.Code, http.StatusNoContent)
	}
}

// ---------------------------------------------------------------------------
// Business logic tests (bypass initData validation)
// ---------------------------------------------------------------------------

func TestBuildMembersResponse(t *testing.T) {
	gen1 := 1
	gen3 := 3

	fq := newFakeQuerier()
	fq.groups = []model.Group{
		{ID: 1, Name: "乃木坂46"},
		{ID: 2, Name: "櫻坂46"},
	}
	fq.members = []model.Member{
		{ID: 10, GroupID: 1, Name: "MemberA", Generation: &gen1},
		{ID: 11, GroupID: 1, Name: "MemberB", Generation: &gen3},
		{ID: 12, GroupID: 1, Name: "DisabledMember", Generation: &gen1, Disabled: true},
		{ID: 20, GroupID: 2, Name: "MemberC", Generation: nil},
	}

	// Subscribe MemberA (10) for chat "42".
	fq.subscribed[10] = map[string]bool{"42": true}

	h := NewHandler(&fakeSender{}, fq, "test-token", "", false)

	members, err := h.buildMembersResponse(context.Background(), "42")
	if err != nil {
		t.Fatalf("buildMembersResponse: %v", err)
	}

	// Should have 3 members (disabled member excluded).
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}

	// Verify first member.
	if members[0].ID != 10 {
		t.Errorf("members[0].ID = %d, want 10", members[0].ID)
	}

	if members[0].Name != "MemberA" {
		t.Errorf("members[0].Name = %q, want MemberA", members[0].Name)
	}

	if members[0].Group != "乃木坂46" {
		t.Errorf("members[0].Group = %q, want 乃木坂46", members[0].Group)
	}

	if members[0].Generation != "一期生" {
		t.Errorf("members[0].Generation = %q, want 一期生", members[0].Generation)
	}

	if !members[0].Subscribed {
		t.Error("members[0].Subscribed should be true")
	}

	// Verify second member (not subscribed).
	if members[1].ID != 11 {
		t.Errorf("members[1].ID = %d, want 11", members[1].ID)
	}

	if members[1].Subscribed {
		t.Error("members[1].Subscribed should be false")
	}

	if members[1].Generation != "三期生" {
		t.Errorf("members[1].Generation = %q, want 三期生", members[1].Generation)
	}

	// Verify third member (nil generation).
	if members[2].ID != 20 {
		t.Errorf("members[2].ID = %d, want 20", members[2].ID)
	}

	if members[2].Group != "櫻坂46" {
		t.Errorf("members[2].Group = %q, want 櫻坂46", members[2].Group)
	}

	if members[2].Generation != "" {
		t.Errorf("members[2].Generation = %q, want empty string", members[2].Generation)
	}
}

func TestApplySubscriptionChangesWithLock(t *testing.T) {
	fq := newFakeQuerier()

	// Pre-acquire lock for chat 42.
	lockID, _ := fq.AcquireLock(context.Background(), "42", 1, "Alice Smith")

	h := NewHandler(&fakeSender{}, fq, "test-token", "", false)

	changes := []subscriptionChange{
		{MemberID: 1, Subscribed: true},
	}

	err := h.applySubscriptionChangesWithLock(context.Background(), "42", lockID, changes)
	if err != nil {
		t.Fatalf("applySubscriptionChangesWithLock: %v", err)
	}

	if !fq.subscribed[1]["42"] {
		t.Error("member 1 should be subscribed for chat 42")
	}

	// Lock should be released.
	holder, _ := fq.GetLockHolder(context.Background(), "42")
	if holder != "" {
		t.Errorf("expected lock released, but holder=%q", holder)
	}
}

func TestApplySubscriptionChangesWithLock_StolenLock(t *testing.T) {
	fq := newFakeQuerier()

	// Alice acquires the lock.
	_, _ = fq.AcquireLock(context.Background(), "42", 1, "Alice Smith")

	h := NewHandler(&fakeSender{}, fq, "test-token", "", false)

	changes := []subscriptionChange{
		{MemberID: 1, Subscribed: true},
	}

	// Bob tries to submit with a stale lock_id.
	err := h.applySubscriptionChangesWithLock(context.Background(), "42", "stale-lock-id", changes)
	if err == nil {
		t.Fatal("expected error when lock held by another user")
	}
}

func TestApplySubscriptionChangesWithLock_InvalidLock(t *testing.T) {
	fq := newFakeQuerier()

	// No lock acquired — submit with a fabricated lock_id.
	h := NewHandler(&fakeSender{}, fq, "test-token", "", false)

	changes := []subscriptionChange{
		{MemberID: 1, Subscribed: true},
	}

	err := h.applySubscriptionChangesWithLock(context.Background(), "42", "fabricated-lock-id", changes)
	if err == nil {
		t.Fatal("expected error for invalid lock_id")
	}

	if err.Error() != "invalid lock" {
		t.Errorf("expected 'invalid lock' error, got: %v", err)
	}

	// Subscription should NOT have been applied.
	if fq.subscribed[1]["42"] {
		t.Error("member 1 should not be subscribed when lock is invalid")
	}
}

func TestApplySubscriptionChangesWithLock_EmptyChanges(t *testing.T) {
	fq := newFakeQuerier()

	// Acquire lock, then submit with empty changes.
	lockID, _ := fq.AcquireLock(context.Background(), "42", 1, "Alice Smith")

	h := NewHandler(&fakeSender{}, fq, "test-token", "", false)

	err := h.applySubscriptionChangesWithLock(context.Background(), "42", lockID, nil)
	if err != nil {
		t.Fatalf("applySubscriptionChangesWithLock with empty changes: %v", err)
	}

	// Lock should still be released even with no changes.
	holder, _ := fq.GetLockHolder(context.Background(), "42")
	if holder != "" {
		t.Errorf("expected lock released after empty changes, but holder=%q", holder)
	}
}

func TestHandleLock_MethodNotAllowed(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/lock", nil)
	w := httptest.NewRecorder()
	h.HandleLock(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/lock: got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleLock_MissingInitData(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/lock", nil)
	w := httptest.NewRecorder()
	h.HandleLock(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/lock without header: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleLock_CORSPreflight(t *testing.T) {
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/lock", nil)
	w := httptest.NewRecorder()
	h.HandleLock(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS /api/lock: got status %d, want %d", w.Code, http.StatusNoContent)
	}
}
