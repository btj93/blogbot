package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	initdata "github.com/telegram-mini-apps/init-data-golang"

	"github.com/btj93/blogbot/observability"
)

// ---------------------------------------------------------------------------
// Auth errors
// ---------------------------------------------------------------------------

var (
	errInitDataMissing = errors.New("missing init data")
	errInitDataInvalid = errors.New("invalid init data")
)

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type apiMember struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Group      string `json:"group"`
	Generation string `json:"generation"`
	Subscribed bool   `json:"subscribed"`
}

type membersResponse struct {
	ChatName string      `json:"chat_name"`
	Members  []apiMember `json:"members"`
}

type subscriptionChange struct {
	MemberID   int64 `json:"member_id"`
	Subscribed bool  `json:"subscribed"`
}

type subscriptionsRequest struct {
	LockID  string               `json:"lock_id"`
	Changes []subscriptionChange `json:"changes"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *Handler) setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Telegram-Init-Data")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type initDataResult struct {
	UserID    int64
	UserName  string
	ChatID    int64
	ChatTitle string
}

type lockResponse struct {
	LockID string `json:"lock_id"`
	Holder string `json:"holder,omitempty"`
}

type lockErrorResponse struct {
	Error  string `json:"error"`
	Holder string `json:"holder"`
}

func initDataErrorCode(err error) string {
	if errors.Is(err, errInitDataMissing) {
		return "not_in_telegram"
	}

	return "invalid_credentials"
}

func (h *Handler) validateInitData(r *http.Request) (initDataResult, error) {
	raw := r.Header.Get("X-Telegram-Init-Data")
	if raw == "" {
		return initDataResult{}, errInitDataMissing
	}

	if err := initdata.Validate(raw, h.botToken, 0); err != nil {
		return initDataResult{}, fmt.Errorf("%w: %w", errInitDataInvalid, err)
	}

	parsed, err := initdata.Parse(raw)
	if err != nil {
		return initDataResult{}, fmt.Errorf("parse init data: %w", err)
	}

	userName := parsed.User.FirstName
	if parsed.User.LastName != "" {
		userName += " " + parsed.User.LastName
	}

	res := initDataResult{
		UserID:    parsed.User.ID,
		UserName:  userName,
		ChatID:    parsed.Chat.ID,
		ChatTitle: parsed.Chat.Title,
	}

	// When opened via direct link (t.me/bot/app?startapp=CHAT_ID),
	// the group chat ID is in StartParam, not Chat.ID.
	if parsed.StartParam != "" {
		if id, err := strconv.ParseInt(parsed.StartParam, 10, 64); err == nil {
			res.ChatID = id
		}
	}

	if res.ChatID == 0 {
		res.ChatID = res.UserID
	}

	return res, nil
}

// ---------------------------------------------------------------------------
// HandleMembers — GET /api/members
// ---------------------------------------------------------------------------

func (h *Handler) HandleMembers(w http.ResponseWriter, r *http.Request) {
	h.setCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	ctx := observability.WithRequestID(r.Context())
	ctx = observability.WithCommand(ctx, "api-members")

	data, err := h.validateInitData(r)
	if err != nil {
		slog.InfoContext(ctx, "init data validation failed", slog.Any("error", err))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: initData validation failed: %v", err))
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: initDataErrorCode(err)})

		return
	}

	if !checkPermission(ctx, h.tg, data.ChatID, data.UserID) {
		slog.InfoContext(ctx, "permission denied",
			slog.Int64("user_id", data.UserID),
			slog.Int64("chat_id", data.ChatID))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: permission denied user=%d chat=%d", data.UserID, data.ChatID))
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "permission_denied"})

		return
	}

	chatIDStr := strconv.FormatInt(data.ChatID, 10)

	members, err := h.buildMembersResponse(ctx, chatIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build members response", slog.Any("error", err))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: failed to build members response: %v", err))
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})

		return
	}

	// Fetch chat title if not in initData (e.g. opened via direct link).
	chatName := data.ChatTitle
	if chatName == "" && data.ChatID != data.UserID {
		if chat, err := h.tg.GetChat(ctx, data.ChatID); err == nil {
			chatName = chat.Title
		}
	}

	writeJSON(w, http.StatusOK, membersResponse{
		ChatName: chatName,
		Members:  members,
	})
}

func (h *Handler) buildMembersResponse(ctx context.Context, chatID string) ([]apiMember, error) {
	groups, err := h.db.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	subIDs, err := h.db.ListSubscribedMemberIDsForChat(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("list subscribed member IDs: %w", err)
	}

	subSet := make(map[int64]bool, len(subIDs))
	for _, id := range subIDs {
		subSet[id] = true
	}

	var result []apiMember

	for _, g := range groups {
		members, err := h.db.ListMembersByGroup(ctx, g.ID)
		if err != nil {
			return nil, fmt.Errorf("list members for group %d: %w", g.ID, err)
		}

		for _, m := range members {
			if m.Disabled {
				continue
			}

			gen := ""
			if m.Generation != nil {
				gen = genLabel(*m.Generation)
			}

			result = append(result, apiMember{
				ID:         m.ID,
				Name:       m.Name,
				Group:      g.Name,
				Generation: gen,
				Subscribed: subSet[m.ID],
			})
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// HandleSubscriptions — POST /api/subscriptions
// ---------------------------------------------------------------------------

func (h *Handler) HandleSubscriptions(w http.ResponseWriter, r *http.Request) {
	h.setCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	ctx := observability.WithRequestID(r.Context())
	ctx = observability.WithCommand(ctx, "api-subscriptions")

	data, err := h.validateInitData(r)
	if err != nil {
		slog.InfoContext(ctx, "init data validation failed", slog.Any("error", err))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: initData validation failed: %v", err))
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: initDataErrorCode(err)})

		return
	}

	if !checkPermission(ctx, h.tg, data.ChatID, data.UserID) {
		slog.InfoContext(ctx, "permission denied",
			slog.Int64("user_id", data.UserID),
			slog.Int64("chat_id", data.ChatID))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: permission denied user=%d chat=%d", data.UserID, data.ChatID))
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "permission_denied"})

		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

	var req subscriptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	chatIDStr := strconv.FormatInt(data.ChatID, 10)

	if len(req.Changes) == 0 {
		if req.LockID != "" {
			if err := h.db.ReleaseLock(ctx, chatIDStr, req.LockID); err != nil {
				slog.WarnContext(ctx, "failed to release lock on empty save", slog.Any("error", err))
			}
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

		return
	}

	if req.LockID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing lock_id"})
		return
	}

	if err := h.applySubscriptionChangesWithLock(ctx, chatIDStr, req.LockID, req.Changes); err != nil {
		if err.Error() == "invalid lock" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid lock_id"})
			return
		}

		// Check if the error is a lock conflict.
		if after, ok := strings.CutPrefix(err.Error(), "locked by "); ok {
			writeJSON(w, http.StatusConflict, lockErrorResponse{Error: "locked", Holder: after})
			return
		}

		slog.ErrorContext(ctx, "failed to apply subscription changes", slog.Any("error", err))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: failed to apply subscription changes: %v", err))
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})

		return
	}

	h.tg.LogText(ctx, fmt.Sprintf("API subscription update\nUser: %d\nChat: %s\nChanges: %d",
		data.UserID, chatIDStr, len(req.Changes)))

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) applySubscriptionChangesWithLock(
	ctx context.Context,
	chatID string,
	lockID string,
	changes []subscriptionChange,
) error {
	tx, err := h.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if tx != nil {
		defer func() { _ = tx.Rollback() }()
	}

	if err := h.db.ValidateAndReleaseLock(ctx, tx, chatID, lockID); err != nil {
		return err
	}

	for _, c := range changes {
		if c.Subscribed {
			if _, err := h.db.AddSubscriptionTx(ctx, tx, c.MemberID, chatID); err != nil {
				return fmt.Errorf("add subscription for member %d: %w", c.MemberID, err)
			}
		} else {
			if _, err := h.db.RemoveSubscriptionTx(ctx, tx, c.MemberID, chatID); err != nil {
				return fmt.Errorf("remove subscription for member %d: %w", c.MemberID, err)
			}
		}
	}

	if tx != nil {
		return tx.Commit()
	}

	return nil
}

// ---------------------------------------------------------------------------
// HandleLock — POST /api/lock
// ---------------------------------------------------------------------------

func (h *Handler) HandleLock(w http.ResponseWriter, r *http.Request) {
	h.setCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	ctx := observability.WithRequestID(r.Context())
	ctx = observability.WithCommand(ctx, "api-lock")

	data, err := h.validateInitData(r)
	if err != nil {
		slog.InfoContext(ctx, "init data validation failed", slog.Any("error", err))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: lock initData validation failed: %v", err))
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: initDataErrorCode(err)})

		return
	}

	if !checkPermission(ctx, h.tg, data.ChatID, data.UserID) {
		slog.InfoContext(ctx, "permission denied",
			slog.Int64("user_id", data.UserID),
			slog.Int64("chat_id", data.ChatID))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: lock permission denied user=%d chat=%d", data.UserID, data.ChatID))
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "permission_denied"})

		return
	}

	chatIDStr := strconv.FormatInt(data.ChatID, 10)

	lockID, err := h.db.AcquireLock(ctx, chatIDStr, data.UserID, data.UserName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to acquire lock", slog.Any("error", err))
		h.tg.LogText(ctx, fmt.Sprintf("webapp: failed to acquire lock: %v", err))
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})

		return
	}

	if lockID == "" {
		// Locked by someone else.
		holder, err := h.db.GetLockHolder(ctx, chatIDStr)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get lock holder", slog.Any("error", err))
			h.tg.LogText(ctx, fmt.Sprintf("webapp: failed to get lock holder: %v", err))
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})

			return
		}

		writeJSON(w, http.StatusConflict, lockErrorResponse{Error: "locked", Holder: holder})

		return
	}

	writeJSON(w, http.StatusOK, lockResponse{LockID: lockID})
}
