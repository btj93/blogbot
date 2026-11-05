package bot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btj93/blogbot/model"
	"github.com/btj93/blogbot/store"
)

func debugRequest(t *testing.T, sender *fakeSender, db store.Querier, message string) string {
	t.Helper()

	h := NewHandler(sender, db, "test-token", "", false)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/",
		strings.NewReader(`{"update_id":1,"message":`+message+`}`),
	)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var text strings.Builder

	for _, sent := range sender.sentTexts {
		if sent.ChatID != sender.logChatID {
			t.Fatalf("debug data sent outside log chat: %d", sent.ChatID)
		}

		_, _ = text.WriteString(sent.Text)
	}

	return text.String()
}

func TestDebugForwardOwnerOnly(t *testing.T) {
	for _, tt := range []struct {
		name                string
		owner, chat, sender int64
		chatType            string
	}{
		{"stranger private chat", 99, 42, 42, "private"},
		{"owner outside log chat", 99, -1001, 99, "supergroup"},
		{"different sender in log chat", 99, 99, 42, "private"},
		{"group log chat is not an owner identity", -1001, -1001, 99, "supergroup"},
		{"unset log chat", 0, 0, 0, "private"},
		{"missing sender", 99, 99, 0, "private"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			from := fmt.Sprintf(`,"from":{"id":%d}`, tt.sender)
			if tt.name == "missing sender" {
				from = ""
			}

			message := fmt.Sprintf(
				`{"chat":{"id":%d,"type":%q}%s,"text":"/debug","forward_origin":{"type":"user","sender_user":{"id":99}}}`,
				tt.chat,
				tt.chatType,
				from,
			)
			fs := &fakeSender{logChatID: tt.owner}

			calls := 0
			if got := debugRequest(t, fs, debugLookupFailure{calls: &calls}, message); got != "" {
				t.Fatalf("unauthorized response: %q", got)
			}

			if calls != 0 {
				t.Fatal("unauthorized message queried subscriptions")
			}
		})
	}
}

func TestDebugForwardOrigins(t *testing.T) {
	for _, tt := range []struct{ name, fields, id, label string }{
		{"modern user", `"forward_origin":{"type":"user","sender_user":{"id":123}}`, "123", "User ID / private chat ID"},
		{"modern group", `"forward_origin":{"type":"chat","sender_chat":{"id":-100123,"type":"supergroup"}}`, "-100123", "Origin chat ID"},
		{"modern channel", `"forward_origin":{"type":"channel","chat":{"id":-100456,"type":"channel"}}`, "-100456", "Origin channel ID"},
		{"legacy user", `"forward_date":100,"forward_from":{"id":123}`, "123", "User ID / private chat ID"},
		{"legacy channel prefers chat over author", `"forward_date":100,"forward_from":{"id":123},"forward_from_chat":{"id":-100456,"type":"channel"}`, "-100456", "Origin chat ID"},
		{"reply command", `"text":"/debug","reply_to_message":{"forward_origin":{"type":"user","sender_user":{"id":123}}}`, "123", "User ID / private chat ID"},
		{"forwarded debug command", `"text":"/debug","forward_origin":{"type":"user","sender_user":{"id":123}},"reply_to_message":{"forward_origin":{"type":"user","sender_user":{"id":999}}}`, "123", "User ID / private chat ID"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fq := newFakeQuerier()
			fq.memberByID[1] = &model.Member{ID: 1, Name: "Test member"}
			fq.memberByID[2] = &model.Member{ID: 2, Name: "Disabled member", Disabled: true}
			fq.subscribed[1] = map[string]bool{tt.id: true}
			fq.subscribed[2] = map[string]bool{tt.id: true}
			fs := &fakeSender{logChatID: 99}

			got := debugRequest(t, fs, fq, `{"chat":{"id":99,"type":"private"},"from":{"id":99},`+tt.fields+`}`)
			for _, want := range []string{tt.label + ": " + tt.id, "Saved subscriptions: 2 (1 enabled, 1 disabled)", "Test member", "Disabled member [disabled: no notifications]"} {
				if !strings.Contains(got, want) {
					t.Errorf("response %q missing %q", got, want)
				}
			}

			if strings.HasPrefix(tt.id, "-") && strings.Contains(got, "private-chat subscriptions only") {
				t.Errorf("group lookup mislabeled as private: %s", got)
			}
		})
	}
}

func TestDebugForwardHiddenOrigin(t *testing.T) {
	for _, fields := range []string{
		`"forward_origin":{"type":"hidden_user","sender_user_name":"u"}`,
		`"forward_date":100,"forward_sender_name":"u"`,
		`"forward_origin":{"type":"hidden_user","sender_user_name":"u"},"forward_from":{"id":123}`,
	} {
		fs := &fakeSender{logChatID: 99}

		got := debugRequest(t, fs, nil, `{"chat":{"id":99,"type":"private"},"from":{"id":99},`+fields+`}`)
		if !strings.Contains(got, "Cannot check subscriptions from a display name") {
			t.Errorf("expected privacy explanation, got %q", got)
		}
	}
}

type debugLookupFailure struct {
	store.Querier
	calls *int
}

func (f debugLookupFailure) ListSubscribedMemberIDsForChat(context.Context, string) ([]int64, error) {
	if f.calls != nil {
		*f.calls++
	}

	return nil, errors.New("database unavailable")
}

func TestDebugForwardEmptyAndErrors(t *testing.T) {
	missingMember := newFakeQuerier()

	missingMember.subscribed[1] = map[string]bool{"123": true}
	for _, tt := range []struct {
		name string
		db   store.Querier
		want string
	}{
		{"no subscriptions", newFakeQuerier(), "No saved subscriptions for this ID"},
		{"database failure", debugLookupFailure{}, "status is unknown (not zero subscriptions)"},
		{"member failure", missingMember, "results are incomplete"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := &fakeSender{logChatID: 99}

			got := debugRequest(
				t,
				fs,
				tt.db,
				`{"chat":{"id":99,"type":"private"},"from":{"id":99},"forward_origin":{"type":"user","sender_user":{"id":123}}}`,
			)
			if !strings.Contains(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDebugForwardNormalMessageAndHelp(t *testing.T) {
	fs := &fakeSender{logChatID: 99}
	if got := debugRequest(t, fs, nil, `{"chat":{"id":99,"type":"private"},"from":{"id":99},"text":"hello"}`); got != "" {
		t.Fatalf("ordinary message triggered debug: %q", got)
	}

	if got := debugRequest(
		t,
		fs,
		nil,
		`{"chat":{"id":99,"type":"private"},"from":{"id":99},"text":"/debug"}`,
	); !strings.Contains(
		got,
		"Forward a message here",
	) {
		t.Fatalf("missing usage: %q", got)
	}
}

func TestDebugLongResponse(t *testing.T) {
	fs := &fakeSender{logChatID: 99}
	h := NewHandler(fs, nil, "test-token", "", false)
	want := strings.Repeat("坂", 8000)
	h.sendDebug(context.Background(), want)

	var got strings.Builder

	for _, sent := range fs.sentTexts {
		if len([]rune(sent.Text)) > 3500 {
			t.Fatal("chunk exceeds limit")
		}

		_, _ = got.WriteString(sent.Text)
	}

	if got.String() != want {
		t.Fatal("chunking lost text")
	}
}
