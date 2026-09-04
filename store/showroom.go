package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/btj93/blogbot/model"
)

func (s *Store) UpsertShowroomRoom(ctx context.Context, memberID int64, roomID, url string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO showroom_rooms (member_id, room_id, url) VALUES ($1, $2, $3)
		 ON CONFLICT(member_id) DO UPDATE SET room_id = excluded.room_id, url = excluded.url,
		 updated_at = NOW()`,
		memberID, roomID, url,
	)
	if err != nil {
		return fmt.Errorf("upserting showroom room: %w", err)
	}

	return nil
}

func (s *Store) UpdateNextLive(ctx context.Context, memberID int64, epoch *int64, text *string) error {
	_, err := s.db.ExecContext(
		ctx,
		"UPDATE showroom_rooms SET next_live_epoch = $1, next_live_text = $2, updated_at = NOW() WHERE member_id = $3",
		epoch,
		text,
		memberID,
	)
	if err != nil {
		return fmt.Errorf("updating next live: %w", err)
	}

	return nil
}

func (s *Store) ListShowroomRoomsWithRoomID(ctx context.Context) ([]model.ShowroomRoom, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT sr.id, sr.member_id, sr.room_id, sr.url, sr.next_live_epoch, sr.next_live_text, sr.created_at, sr.updated_at FROM showroom_rooms sr JOIN members m ON m.id = sr.member_id WHERE sr.room_id != '' AND m.disabled = FALSE",
	)
	if err != nil {
		return nil, fmt.Errorf("listing showroom rooms: %w", err)
	}
	defer rows.Close()

	return scanShowroomRooms(rows)
}

func (s *Store) ListShowroomRoomsWithURL(ctx context.Context) ([]model.ShowroomRoom, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT sr.id, sr.member_id, sr.room_id, sr.url, sr.next_live_epoch, sr.next_live_text, sr.created_at, sr.updated_at FROM showroom_rooms sr JOIN members m ON m.id = sr.member_id WHERE sr.url != '' AND m.disabled = FALSE",
	)
	if err != nil {
		return nil, fmt.Errorf("listing showroom rooms with URL: %w", err)
	}
	defer rows.Close()

	return scanShowroomRooms(rows)
}

func scanShowroomRooms(rows *sql.Rows) ([]model.ShowroomRoom, error) {
	var rooms []model.ShowroomRoom

	for rows.Next() {
		var (
			r     model.ShowroomRoom
			epoch sql.NullInt64
			text  sql.NullString
		)

		if err := rows.Scan(&r.ID, &r.MemberID, &r.RoomID, &r.URL, &epoch, &text, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}

		if epoch.Valid {
			r.NextLiveEpoch = &epoch.Int64
		}

		if text.Valid {
			r.NextLiveText = &text.String
		}

		rooms = append(rooms, r)
	}

	return rooms, rows.Err()
}
