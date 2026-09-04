package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/model"
)

// ---------------------------------------------------------------------------
// Existing pure-function tests
// ---------------------------------------------------------------------------

func TestIsCommand(t *testing.T) {
	tests := []struct {
		text string
		cmd  string
		want bool
	}{
		{"/start", "start", true},
		{"/start@NogiBlog_bot", "start", true},
		{"/start something", "start", true},
		{"/help", "help", true},
		{"/editsublist", "editsublist", true},
		{"/other", "start", false},
		{"hello", "start", false},
	}
	for _, tt := range tests {
		got := isCommand(tt.text, tt.cmd)
		if got != tt.want {
			t.Errorf("isCommand(%q, %q) = %v, want %v", tt.text, tt.cmd, got, tt.want)
		}
	}
}

func TestGenLabel(t *testing.T) {
	tests := []struct {
		gen  int
		want string
	}{
		{1, "一期生"},
		{4, "四期生"},
		{5, "五期生"},
		{10, "十期生"},
	}
	for _, tt := range tests {
		got := genLabel(tt.gen)
		if got != tt.want {
			t.Errorf("genLabel(%d) = %q, want %q", tt.gen, got, tt.want)
		}
	}
}

func TestParseGeneration(t *testing.T) {
	if g := parseGeneration("null"); g != nil {
		t.Errorf("parseGeneration(null) = %v, want nil", g)
	}

	if g := parseGeneration("4"); g == nil || *g != 4 {
		t.Errorf("parseGeneration(4) = %v, want 4", g)
	}
}

// ---------------------------------------------------------------------------
// Task 14: HTTP layer tests
// ---------------------------------------------------------------------------

func TestServeHTTP_RejectNonPOST(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	h := NewHandler(fs, fq, "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET request: got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeHTTP_MalformedJSON(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	h := NewHandler(fs, fq, "test-token", "", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON: got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestServeHTTP_ValidUpdate(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	h := NewHandler(fs, fq, "test-token", "", false)

	// Minimal valid Update JSON (empty update, no message/callback).
	body := `{"update_id":12345}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid update: got status %d, want %d", w.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// Task 15: Command and callback routing tests
// ---------------------------------------------------------------------------

func TestHandleHelp_Private(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	h := NewHandler(fs, fq, "test-token", "", false)

	// Simulate a /help command in a private chat.
	body := `{
		"update_id": 100,
		"message": {
			"message_id": 1,
			"from": {"id": 42, "first_name": "Test"},
			"chat": {"id": 42, "type": "private"},
			"text": "/help"
		}
	}`

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("help command: got status %d, want %d", w.Code, http.StatusOK)
	}

	if len(fs.sentTexts) != 1 {
		t.Fatalf("expected 1 sent text, got %d", len(fs.sentTexts))
	}

	sent := fs.sentTexts[0]
	if sent.ChatID != 42 {
		t.Errorf("sent to chatID %d, want 42", sent.ChatID)
	}

	if !strings.Contains(sent.Text, "/help") {
		t.Errorf("help text should contain /help, got %q", sent.Text)
	}
}

func TestCallback_GroupSelection(t *testing.T) {
	// Reset admin cache before and after.
	adminCache.mu.Lock()
	adminCache.cache = make(map[int64]adminEntry)
	adminCache.mu.Unlock()
	t.Cleanup(func() {
		adminCache.mu.Lock()
		adminCache.cache = make(map[int64]adminEntry)
		adminCache.mu.Unlock()
	})

	fs := &fakeSender{
		adminMap: map[int64][]tgbotapi.ChatMember{
			-100: {
				{User: &tgbotapi.User{ID: 42}},
			},
		},
	}

	gen1 := 1
	gen2 := 2
	fq := newFakeQuerier()
	fq.groups = []model.Group{
		{ID: 1, Name: "乃木坂46"},
		{ID: 2, Name: "櫻坂46"},
	}
	fq.generations = []*int{&gen1, &gen2}

	h := NewHandler(fs, fq, "test-token", "", false)

	// Callback query: user 42 in group chat -100 selects group 1.
	body := `{
		"update_id": 200,
		"callback_query": {
			"id": "abc",
			"from": {"id": 42, "first_name": "Test", "last_name": "", "username": "tester"},
			"message": {
				"message_id": 10,
				"chat": {"id": -100, "type": "group", "title": "TestGroup"},
				"text": "old text"
			},
			"data": "g:1"
		}
	}`

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("callback g:1: got status %d, want %d", w.Code, http.StatusOK)
	}

	if len(fs.edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(fs.edits))
	}

	edit := fs.edits[0]
	if edit.ChatID != -100 {
		t.Errorf("edit chatID = %d, want -100", edit.ChatID)
	}

	if edit.MessageID != 10 {
		t.Errorf("edit messageID = %d, want 10", edit.MessageID)
	}
}

func TestCallback_ToggleSubscription(t *testing.T) {
	// Reset admin cache.
	adminCache.mu.Lock()
	adminCache.cache = make(map[int64]adminEntry)
	adminCache.mu.Unlock()
	t.Cleanup(func() {
		adminCache.mu.Lock()
		adminCache.cache = make(map[int64]adminEntry)
		adminCache.mu.Unlock()
	})

	gen4 := 4
	fq := newFakeQuerier()
	fq.memberByID[5] = &model.Member{
		ID:         5,
		GroupID:    1,
		Name:       "TestMember",
		Generation: &gen4,
	}
	fq.members = []model.Member{
		{ID: 5, GroupID: 1, Name: "TestMember", Generation: &gen4},
	}

	fs := &fakeSender{}

	h := NewHandler(fs, fq, "test-token", "", false)

	// Private chat (chatID == userID), so permission is always granted.
	body := `{
		"update_id": 300,
		"callback_query": {
			"id": "def",
			"from": {"id": 42, "first_name": "Test", "last_name": "", "username": "tester"},
			"message": {
				"message_id": 20,
				"chat": {"id": 42, "type": "private"},
				"text": "old text"
			},
			"data": "t:5"
		}
	}`

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("callback t:5: got status %d, want %d", w.Code, http.StatusOK)
	}

	// The toggle should have called AddSubscription (member was not subscribed).
	if !fq.subscribed[5]["42"] {
		t.Error("expected member 5 to be subscribed for chat 42 after toggle")
	}

	// Should have sent an edit with the updated keyboard.
	if len(fs.edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(fs.edits))
	}
}

// ---------------------------------------------------------------------------
// Task 16: Admin cache tests (must NOT use t.Parallel)
// ---------------------------------------------------------------------------

func TestAdminCache_PrivateChatBypass(t *testing.T) {
	adminCache.mu.Lock()
	adminCache.cache = make(map[int64]adminEntry)
	adminCache.mu.Unlock()
	t.Cleanup(func() {
		adminCache.mu.Lock()
		adminCache.cache = make(map[int64]adminEntry)
		adminCache.mu.Unlock()
	})

	fs := &fakeSender{}

	// chatID == userID => private chat => always permitted.
	if !checkPermission(context.Background(), fs, 42, 42) {
		t.Error("checkPermission(42, 42) should return true for private chat")
	}
}

func TestAdminCache_CacheHit(t *testing.T) {
	adminCache.mu.Lock()
	adminCache.cache = make(map[int64]adminEntry)
	adminCache.mu.Unlock()
	t.Cleanup(func() {
		adminCache.mu.Lock()
		adminCache.cache = make(map[int64]adminEntry)
		adminCache.mu.Unlock()
	})

	fs := &fakeSender{
		adminMap: map[int64][]tgbotapi.ChatMember{
			-200: {
				{User: &tgbotapi.User{ID: 42}},
			},
		},
	}

	// First call: should query the API and cache the result.
	if !checkPermission(context.Background(), fs, -200, 42) {
		t.Fatal("first call: expected true for admin user 42")
	}

	// Remove the admin map to simulate API unavailability. The cache should
	// still return the correct result.
	fs.adminMap = nil

	if !checkPermission(context.Background(), fs, -200, 42) {
		t.Error("second call: expected true from cache for admin user 42")
	}
}

func TestAdminCache_NonAdmin(t *testing.T) {
	adminCache.mu.Lock()
	adminCache.cache = make(map[int64]adminEntry)
	adminCache.mu.Unlock()
	t.Cleanup(func() {
		adminCache.mu.Lock()
		adminCache.cache = make(map[int64]adminEntry)
		adminCache.mu.Unlock()
	})

	fs := &fakeSender{
		adminMap: map[int64][]tgbotapi.ChatMember{
			-300: {
				{User: &tgbotapi.User{ID: 99}}, // only user 99 is admin
			},
		},
	}

	// User 42 is not in the admin list.
	if checkPermission(context.Background(), fs, -300, 42) {
		t.Error("expected false for non-admin user 42")
	}
}

// ---------------------------------------------------------------------------
// WebApp mode tests
// ---------------------------------------------------------------------------

func TestEditSublist_WebAppMode(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	h := NewHandler(fs, fq, "test-token", "https://example.com/webapp", true)

	body := `{
		"update_id": 500,
		"message": {
			"message_id": 1,
			"from": {"id": 42, "first_name": "Test"},
			"chat": {"id": 42, "type": "private"},
			"text": "/editsublist"
		}
	}`

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if len(fs.sentTexts) != 1 {
		t.Fatalf("expected 1 sent text, got %d", len(fs.sentTexts))
	}

	sent := fs.sentTexts[0]
	if sent.ChatID != 42 {
		t.Errorf("sent to chatID %d, want 42", sent.ChatID)
	}

	if sent.Opts == nil || sent.Opts.ReplyMarkup == nil {
		t.Fatal("expected reply markup with WebApp URL button")
	}
}

func TestEditSublist_InlineKeyboardMode(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	fq.groups = []model.Group{
		{ID: 1, Name: "乃木坂46"},
	}
	h := NewHandler(fs, fq, "test-token", "", false)

	body := `{
		"update_id": 501,
		"message": {
			"message_id": 1,
			"from": {"id": 42, "first_name": "Test"},
			"chat": {"id": 42, "type": "private"},
			"text": "/editsublist"
		}
	}`

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if len(fs.sentTexts) != 1 {
		t.Fatalf("expected 1 sent text, got %d", len(fs.sentTexts))
	}

	// In inline keyboard mode, the reply markup should be a group keyboard (not URL button).
	sent := fs.sentTexts[0]
	if sent.Opts == nil || sent.Opts.ReplyMarkup == nil {
		t.Fatal("expected reply markup with inline keyboard")
	}
}

func TestStaleCallback_WebAppMode(t *testing.T) {
	fs := &fakeSender{}
	fq := newFakeQuerier()
	h := NewHandler(fs, fq, "test-token", "https://example.com/webapp", true)

	body := `{
		"update_id": 502,
		"callback_query": {
			"id": "stale-cb-123",
			"from": {"id": 42, "first_name": "Test", "last_name": "", "username": "tester"},
			"message": {
				"message_id": 10,
				"chat": {"id": 42, "type": "private"},
				"text": "old text"
			},
			"data": "g:1"
		}
	}`

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if len(fs.callbackAnswers) != 1 {
		t.Fatalf("expected 1 callback answer, got %d", len(fs.callbackAnswers))
	}

	answer := fs.callbackAnswers[0]
	if answer.CallbackQueryID != "stale-cb-123" {
		t.Errorf("callback query ID = %q, want stale-cb-123", answer.CallbackQueryID)
	}

	if answer.Text != "請使用 /editsublist 開啟新版訂閱管理" {
		t.Errorf("callback answer text = %q, want redirect message", answer.Text)
	}

	// Should NOT have processed the callback normally (no edits).
	if len(fs.edits) != 0 {
		t.Errorf("expected 0 edits in webapp mode, got %d", len(fs.edits))
	}
}
