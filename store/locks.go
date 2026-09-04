package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// AcquireLock attempts to acquire the webapp edit lock for a chat.
// Returns the lock_id on success, or empty string if locked by another user.
func (s *Store) AcquireLock(ctx context.Context, chatID string, userID int64, userName string) (string, error) {
	// Delete expired locks for this chat.
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM webapp_locks WHERE chat_id = $1 AND expires_at < NOW()",
		chatID,
	)
	if err != nil {
		return "", fmt.Errorf("deleting expired lock: %w", err)
	}

	// Try to insert or update if same user.
	var lockID string

	err = s.db.QueryRowContext(ctx, `
		INSERT INTO webapp_locks (chat_id, user_id, user_name, lock_id, expires_at)
		VALUES ($1, $2, $3, gen_random_uuid(), NOW() + INTERVAL '5 minutes')
		ON CONFLICT (chat_id) DO UPDATE
		  SET lock_id = gen_random_uuid(),
		      user_id = EXCLUDED.user_id,
		      user_name = EXCLUDED.user_name,
		      expires_at = NOW() + INTERVAL '5 minutes',
		      created_at = NOW()
		  WHERE webapp_locks.user_id = EXCLUDED.user_id
		RETURNING lock_id`,
		chatID, userID, userName,
	).Scan(&lockID)

	if errors.Is(err, sql.ErrNoRows) {
		// Conflict — another user holds the lock.
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("acquiring lock: %w", err)
	}

	return lockID, nil
}

// GetLockHolder returns the user_name of the current lock holder for a chat.
// Returns empty string if no active (non-expired) lock exists.
func (s *Store) GetLockHolder(ctx context.Context, chatID string) (string, error) {
	var userName string

	err := s.db.QueryRowContext(ctx,
		"SELECT user_name FROM webapp_locks WHERE chat_id = $1 AND expires_at >= NOW()",
		chatID,
	).Scan(&userName)

	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("getting lock holder: %w", err)
	}

	return userName, nil
}

// ReleaseLock deletes a lock by chat_id and lock_id without requiring a transaction.
// Used when releasing a lock with no subscription changes (e.g. save with empty changes).
func (s *Store) ReleaseLock(ctx context.Context, chatID string, lockID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM webapp_locks WHERE chat_id = $1 AND lock_id = $2",
		chatID, lockID,
	)
	if err != nil {
		return fmt.Errorf("releasing lock: %w", err)
	}

	return nil
}

// ValidateAndReleaseLock validates the lock_id and deletes the lock within a transaction.
// If the lock_id matches (even if expired), the lock is released and nil is returned.
// If the lock_id doesn't match and someone else holds an active lock, returns "locked by <name>".
// If the lock_id doesn't match and no active lock exists, returns "invalid lock".
func (s *Store) ValidateAndReleaseLock(ctx context.Context, tx *sql.Tx, chatID string, lockID string) error {
	// Try to delete the matching lock (works even if expired).
	res, err := tx.ExecContext(ctx,
		"DELETE FROM webapp_locks WHERE chat_id = $1 AND lock_id = $2",
		chatID, lockID,
	)
	if err != nil {
		return fmt.Errorf("deleting lock: %w", err)
	}

	n, _ := res.RowsAffected()
	if n > 0 {
		// lock_id matched — success.
		return nil
	}

	// lock_id didn't match. Check who holds the lock now.
	var userName string

	err = tx.QueryRowContext(ctx,
		"SELECT user_name FROM webapp_locks WHERE chat_id = $1 AND expires_at >= NOW()",
		chatID,
	).Scan(&userName)

	if errors.Is(err, sql.ErrNoRows) {
		// No active lock, but lock_id also didn't match — invalid.
		return errors.New("invalid lock")
	}

	if err != nil {
		return fmt.Errorf("checking lock holder: %w", err)
	}

	// Someone else holds it.
	return fmt.Errorf("locked by %s", userName)
}

// BeginTx starts a new database transaction.
func (s *Store) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}
