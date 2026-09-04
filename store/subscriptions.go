package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) AddSubscription(ctx context.Context, memberID int64, chatID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO subscriptions (member_id, chat_id) VALUES ($1, $2) ON CONFLICT (member_id, chat_id) DO NOTHING",
		memberID, chatID,
	)
	if err != nil {
		return false, fmt.Errorf("adding subscription: %w", err)
	}

	n, _ := res.RowsAffected()

	return n > 0, nil
}

func (s *Store) RemoveSubscription(ctx context.Context, memberID int64, chatID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM subscriptions WHERE member_id = $1 AND chat_id = $2",
		memberID, chatID,
	)
	if err != nil {
		return false, fmt.Errorf("removing subscription: %w", err)
	}

	n, _ := res.RowsAffected()

	return n > 0, nil
}

func (s *Store) IsSubscribed(ctx context.Context, memberID int64, chatID string) (bool, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM subscriptions WHERE member_id = $1 AND chat_id = $2",
		memberID, chatID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking subscription: %w", err)
	}

	return count > 0, nil
}

func (s *Store) GetSubscriberChatIDs(ctx context.Context, memberID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT chat_id FROM subscriptions WHERE member_id = $1 ORDER BY chat_id",
		memberID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting subscribers: %w", err)
	}
	defer rows.Close()

	var chatIDs []string

	for rows.Next() {
		var chatID string
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}

		chatIDs = append(chatIDs, chatID)
	}

	return chatIDs, rows.Err()
}

func (s *Store) ListSubscribedMemberIDsForChat(ctx context.Context, chatID string) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT member_id FROM subscriptions WHERE chat_id = $1 ORDER BY member_id",
		chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing subscribed member IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func (s *Store) AddSubscriptionForAllInGeneration(
	ctx context.Context,
	groupID int64,
	generation *int,
	chatID string,
) error {
	var err error
	if generation == nil {
		_, err = s.db.ExecContext(
			ctx,
			"INSERT INTO subscriptions (member_id, chat_id) SELECT id, $1 FROM members WHERE group_id = $2 AND generation IS NULL AND disabled = FALSE ON CONFLICT (member_id, chat_id) DO NOTHING",
			chatID,
			groupID,
		)
	} else {
		_, err = s.db.ExecContext(
			ctx,
			"INSERT INTO subscriptions (member_id, chat_id) SELECT id, $1 FROM members WHERE group_id = $2 AND generation = $3 AND disabled = FALSE ON CONFLICT (member_id, chat_id) DO NOTHING",
			chatID,
			groupID,
			*generation,
		)
	}

	if err != nil {
		return fmt.Errorf("adding all subscriptions: %w", err)
	}

	return nil
}

func (s *Store) AddSubscriptionTx(ctx context.Context, tx *sql.Tx, memberID int64, chatID string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		"INSERT INTO subscriptions (member_id, chat_id) VALUES ($1, $2) ON CONFLICT (member_id, chat_id) DO NOTHING",
		memberID, chatID,
	)
	if err != nil {
		return false, fmt.Errorf("adding subscription: %w", err)
	}

	n, _ := res.RowsAffected()

	return n > 0, nil
}

func (s *Store) RemoveSubscriptionTx(ctx context.Context, tx *sql.Tx, memberID int64, chatID string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		"DELETE FROM subscriptions WHERE member_id = $1 AND chat_id = $2",
		memberID, chatID,
	)
	if err != nil {
		return false, fmt.Errorf("removing subscription: %w", err)
	}

	n, _ := res.RowsAffected()

	return n > 0, nil
}

func (s *Store) RemoveSubscriptionForAllInGeneration(
	ctx context.Context,
	groupID int64,
	generation *int,
	chatID string,
) error {
	var err error
	if generation == nil {
		_, err = s.db.ExecContext(
			ctx,
			"DELETE FROM subscriptions WHERE chat_id = $1 AND member_id IN (SELECT id FROM members WHERE group_id = $2 AND generation IS NULL AND disabled = FALSE)",
			chatID,
			groupID,
		)
	} else {
		_, err = s.db.ExecContext(
			ctx,
			"DELETE FROM subscriptions WHERE chat_id = $1 AND member_id IN (SELECT id FROM members WHERE group_id = $2 AND generation = $3 AND disabled = FALSE)",
			chatID,
			groupID,
			*generation,
		)
	}

	if err != nil {
		return fmt.Errorf("removing all subscriptions: %w", err)
	}

	return nil
}
