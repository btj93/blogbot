package showroom

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/btj93/blogbot/model"
	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

const (
	wsPingInterval = 60 * time.Second
	wsPongTimeout  = 90 * time.Second
)

type wsMessage struct {
	T int `json:"t"`
}

func MonitorLive(ctx context.Context, db store.Querier, tg telegram.Sender, logOnly bool) error {
	rooms, err := db.ListShowroomRoomsWithURL(ctx)
	if err != nil {
		return fmt.Errorf("listing rooms: %w", err)
	}

	type roomWithMember struct {
		room   model.ShowroomRoom
		member *model.Member
	}

	var targets []roomWithMember

	for _, room := range rooms {
		member, err := db.GetMemberByID(ctx, room.MemberID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get member", slog.Int64("member_id", room.MemberID), slog.Any("error", err))
			tg.LogText(ctx, fmt.Sprintf("Failed to get member %d: %v", room.MemberID, err))

			continue
		}

		slog.InfoContext(
			ctx,
			"loaded member for monitoring",
			slog.String("member", member.Name),
			slog.String("url", room.URL),
		)
		targets = append(targets, roomWithMember{room: room, member: member})
	}

	slog.InfoContext(ctx, "starting live monitors", slog.Int("targets", len(targets)))
	tg.LogText(ctx, fmt.Sprintf("Starting live monitors for %d members", len(targets)))

	for _, t := range targets {
		go monitorOne(ctx, db, tg, t.room, t.member, logOnly)
	}

	<-ctx.Done()

	return ctx.Err()
}

func monitorOne(
	ctx context.Context,
	db store.Querier,
	tg telegram.Sender,
	room model.ShowroomRoom,
	member *model.Member,
	logOnly bool,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		current, err := db.GetMemberByID(ctx, member.ID)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"failed to refresh member, continuing",
				slog.String("member", member.Name),
				slog.Any("error", err),
			)
			tg.LogText(ctx, fmt.Sprintf("%s failed to refresh member: %v", member.Name, err))
		} else if current.Disabled {
			slog.InfoContext(ctx, "member disabled, stopping monitor", slog.String("member", member.Name))
			tg.LogText(ctx, fmt.Sprintf("%s disabled, stopping monitor", member.Name))

			return
		}

		err = monitorOneLoop(ctx, db, tg, room, member, logOnly)
		if err != nil {
			slog.ErrorContext(ctx, "monitor error", slog.String("member", member.Name), slog.Any("error", err))
			tg.LogText(ctx, fmt.Sprintf("%s monitor error: %v", member.Name, err))
			observability.WSReconnects.Add(1)
			observability.PromWSReconnects.Inc()
			sleepCtx(ctx, 60*time.Second)
		}
		// Normal exit from inner loop: immediately retry (no sleep)
	}
}

func monitorOneLoop(
	ctx context.Context,
	db store.Querier,
	tg telegram.Sender,
	room model.ShowroomRoom,
	member *model.Member,
	logOnly bool,
) error {
	status, err := FetchRoomStatus(ctx, room.URL)
	if err != nil || status.BroadcastHost == "" {
		slog.WarnContext(
			ctx,
			"fetch room status failed, retrying in 10 minutes",
			slog.String("member", member.Name),
			slog.Any("error", err),
		)
		tg.LogText(ctx, fmt.Sprintf("%s Failed. Retrying in 10 minutes", member.Name))
		sleepCtx(ctx, 10*time.Minute)

		return nil
	}

	if strings.Contains(status.BroadcastKey, ":") {
		slog.InfoContext(ctx, "member already live", slog.String("member", member.Name))
		tg.LogText(ctx, fmt.Sprintf("%s now live", member.Name))
		sleepCtx(ctx, 60*time.Second)

		return nil
	}

	wsURL := fmt.Sprintf("wss://%s/", status.BroadcastHost)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	defer conn.Close()

	subMsg := fmt.Sprintf("SUB\t%s", status.BroadcastKey)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subMsg)); err != nil {
		return fmt.Errorf("ws send: %w", err)
	}

	// Set up ping/pong keepalive
	_ = conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
		return nil
	})

	// Start ping ticker in background
	pingDone := make(chan struct{})

	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-pingDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	defer close(pingDone)

	slog.InfoContext(ctx, "websocket connected", slog.String("member", member.Name))
	observability.WSConnections.Add(1)
	observability.PromWSConnections.Inc()
	tg.LogText(ctx, fmt.Sprintf("WebSocket Connected to %s", member.Name))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("ws read: %w", err)
		}

		slog.DebugContext(ctx, "websocket message", slog.String("member", member.Name), slog.String("msg", string(msg)))

		// Messages arrive as MSG\t<broadcast_key>\t<json> (3 tab-separated fields).
		// Extract the last field which contains the JSON payload.
		parts := strings.Split(string(msg), "\t")
		if len(parts) < 2 {
			continue
		}

		var wsMsg wsMessage
		if err := json.Unmarshal([]byte(parts[len(parts)-1]), &wsMsg); err != nil {
			continue
		}

		switch wsMsg.T {
		case 104: // Stream started
			slog.InfoContext(ctx, "stream started", slog.String("member", member.Name))
			tg.LogText(ctx, fmt.Sprintf("%s stream started", member.Name))
			observability.StreamStarts.Add(1)
			observability.PromStreamStarts.Inc()

			m3u8, err := FetchStreamingURL(ctx, room.RoomID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to fetch m3u8", slog.String("member", member.Name), slog.Any("error", err))
			}

			msgText := fmt.Sprintf("#%s is now Live!\n%s\n\n%s", member.Name, room.URL, m3u8)

			if logOnly {
				tg.LogText(ctx, msgText)
				slog.InfoContext(ctx, "log-only mode, skipping subscriber notifications",
					slog.String("member", member.Name))
			} else {
				chatIDs, _ := db.GetSubscriberChatIDs(ctx, member.ID)
				for _, chatID := range chatIDs {
					cid := parseChatID(chatID)

					if err := tg.SendText(ctx, cid, msgText, nil); err != nil {
						slog.ErrorContext(
							ctx,
							"failed to send live notification",
							slog.String("member", member.Name),
							slog.Any("error", err),
						)
					}
				}
			}

			return nil

		case 101: // Stream ended
			slog.InfoContext(ctx, "stream ended", slog.String("member", member.Name))
			tg.LogText(ctx, fmt.Sprintf("%s stream ended", member.Name))
			sleepCtx(ctx, 5*time.Minute)

			return nil
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
