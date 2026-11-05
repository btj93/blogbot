package bot

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/store"
)

func captureDiagnosticLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(
		slog.New(
			observability.NewContextHandler(
				slog.NewJSONHandler(&logs, nil),
				[]observability.ContextKey{observability.RequestIDKey{}, observability.CommandKey{}},
			),
		),
	)
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &logs
}

func TestCommandDiagnostics(t *testing.T) {
	logs := captureDiagnosticLogs(t)
	h := NewHandler(&fakeSender{}, newFakeQuerier(), "test-token", "", true)

	for _, command := range []string{"start", "help", "editsublist"} {
		body := `{"message":{"from":{"id":42},"chat":{"id":42,"type":"private"},"text":"/` + command + ` secret-command-argument"}}`
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body)))

		if w.Code != http.StatusOK {
			t.Fatalf("status=%d", w.Code)
		}
	}

	if strings.Contains(logs.String(), "secret-command-argument") {
		t.Fatal("command body leaked into logs")
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three command events: %s", logs.String())
	}

	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}

		if record["msg"] != "bot command received" || record["user_id"] != float64(42) ||
			record["chat_id"] != float64(42) ||
			record["chat_type"] != "private" ||
			record["request_id"] == nil ||
			record["bot_command"] == nil {
			t.Errorf("incomplete command event: %s", line)
		}
	}
}

// This driver only supplies a real sql.Tx boundary; fake mutation methods below
// let the tests independently reject a write or the commit.
type auditDriver struct{ commitErr error }

func (d auditDriver) Open(string) (driver.Conn, error)             { return auditConn(d), nil }
func (d auditDriver) Connect(context.Context) (driver.Conn, error) { return d.Open("") }
func (d auditDriver) Driver() driver.Driver                        { return d }

type auditConn auditDriver

func (c auditConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected SQL statement")
}
func (auditConn) Close() error                { return nil }
func (c auditConn) Begin() (driver.Tx, error) { return auditTx(c), nil }

type auditTx auditDriver

func (tx auditTx) Commit() error { return tx.commitErr }
func (auditTx) Rollback() error  { return nil }

type auditQuerier struct {
	store.Querier
	db             *sql.DB
	failAt, writes int
	unchanged      bool
}

func (q *auditQuerier) BeginTx(ctx context.Context) (*sql.Tx, error) { return q.db.BeginTx(ctx, nil) }
func (*auditQuerier) ValidateAndReleaseLock(context.Context, *sql.Tx, string, string) error {
	return nil
}
func (q *auditQuerier) AddSubscriptionTx(context.Context, *sql.Tx, int64, string) (bool, error) {
	q.writes++
	if q.failAt == q.writes {
		return false, errors.New("write rejected")
	}

	return !q.unchanged, nil
}

func (q *auditQuerier) RemoveSubscriptionTx(
	ctx context.Context,
	tx *sql.Tx,
	memberID int64,
	chatID string,
) (bool, error) {
	return q.AddSubscriptionTx(ctx, tx, memberID, chatID)
}

func TestSubscriptionAuditOnlyAfterCommit(t *testing.T) {
	for _, tt := range []struct {
		name       string
		failAt     int
		commitErr  error
		unchanged  bool
		wantEvents int
		wantErr    bool
	}{
		{name: "committed changes", wantEvents: 2},
		{name: "write failure rolls back", failAt: 2, wantErr: true},
		{name: "commit failure", commitErr: errors.New("commit rejected"), wantErr: true},
		{name: "no changes", unchanged: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureDiagnosticLogs(t)
			db := sql.OpenDB(auditDriver{commitErr: tt.commitErr})

			t.Cleanup(func() { _ = db.Close() })

			q := &auditQuerier{db: db, failAt: tt.failAt, unchanged: tt.unchanged}
			h := NewHandler(&fakeSender{}, q, "test-token", "", true)
			ctx := observability.WithAttrs(observability.WithRequestID(context.Background()), slog.Int64("user_id", 99))

			err := h.applySubscriptionChangesWithLock(
				ctx,
				"42",
				"lock",
				[]subscriptionChange{{MemberID: 1, Subscribed: true}, {MemberID: 2, Subscribed: false}},
			)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := strings.Count(logs.String(), `"msg":"subscription changed"`); got != tt.wantEvents {
				t.Fatalf("events=%d, want %d: %s", got, tt.wantEvents, logs.String())
			}

			if tt.wantEvents > 0 {
				for _, want := range []string{`"user_id":99`, `"chat_id":"42"`, `"member_id":1`, `"subscribed":true`, `"member_id":2`, `"subscribed":false`, `"request_id":`} {
					if !strings.Contains(logs.String(), want) {
						t.Errorf("missing %s: %s", want, logs.String())
					}
				}
			}
		})
	}
}
