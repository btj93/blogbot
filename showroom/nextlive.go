package showroom

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/btj93/blogbot/model"
	"github.com/btj93/blogbot/observability"
	"github.com/btj93/blogbot/store"
	"github.com/btj93/blogbot/telegram"
)

type nextLiveResult struct {
	room    model.ShowroomRoom
	resp    *NextLiveResponse
	changed bool
}

func CheckNextLive(ctx context.Context, db store.Querier, tg telegram.Sender, logOnly bool) error {
	rooms, err := db.ListShowroomRoomsWithRoomID(ctx)
	if err != nil {
		return fmt.Errorf("listing rooms: %w", err)
	}

	// Phase 1: Fetch all next_live data concurrently
	sem := semaphore.NewWeighted(5)
	g, gctx := errgroup.WithContext(ctx)

	var (
		mu      sync.Mutex
		results []nextLiveResult
	)

	for _, room := range rooms {
		g.Go(func() error {
			if err := sem.Acquire(gctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			resp, err := FetchNextLive(gctx, room.RoomID)
			if err != nil {
				slog.ErrorContext(
					gctx,
					"failed to fetch next_live",
					slog.String("room_id", room.RoomID),
					slog.Any("error", err),
				)
				observability.NextLiveErrors.Add(1)

				return nil
			}

			changed := isNextLiveChanged(room.NextLiveEpoch, room.NextLiveText, resp.Epoch, resp.Text)

			mu.Lock()

			results = append(results, nextLiveResult{room: room, resp: resp, changed: changed})
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Phase 2: Notify and update DB sequentially (no lock contention)
	for _, r := range results {
		if r.changed && r.resp.Text != "TBD" {
			member, err := db.GetMemberByID(ctx, r.room.MemberID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to get member",
					slog.Int64("member_id", r.room.MemberID), slog.Any("error", err))
			} else {
				msg := fmt.Sprintf("#%s\nNext Live: %s JST\n\n%s", member.Name, r.resp.Text, r.room.URL)

				if logOnly {
					tg.LogText(ctx, msg)
					slog.InfoContext(ctx, "log-only mode, skipping subscriber notifications",
						slog.String("member", member.Name))
				} else {
					chatIDs, _ := db.GetSubscriberChatIDs(ctx, member.ID)
					for _, chatID := range chatIDs {
						cid := parseChatID(chatID)

						if err := tg.SendText(ctx, cid, msg, nil); err != nil {
							slog.ErrorContext(ctx, "failed to send next_live notification", slog.Any("error", err))
						}
					}
				}
			}
		}

		var textPtr *string
		if r.resp.Text != "" {
			textPtr = &r.resp.Text
		}

		if err := db.UpdateNextLive(ctx, r.room.MemberID, r.resp.Epoch, textPtr); err != nil {
			slog.ErrorContext(ctx, "failed to update next_live", slog.Any("error", err))
		}
	}

	return nil
}

func isNextLiveChanged(oldEpoch *int64, oldText *string, newEpoch *int64, newText string) bool {
	if oldEpoch == nil && newEpoch != nil {
		return true
	}

	if oldEpoch != nil && newEpoch == nil {
		return true
	}

	if oldEpoch != nil && newEpoch != nil && *oldEpoch != *newEpoch {
		return true
	}

	if oldText == nil && newText != "" {
		return true
	}

	if oldText != nil && *oldText != newText {
		return true
	}

	return false
}

func parseChatID(s string) int64 {
	var n int64

	_, _ = fmt.Sscanf(s, "%d", &n)

	return n
}
