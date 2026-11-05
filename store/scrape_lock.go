package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

const scrapeLockID int64 = 0x626c6f67626f74

// TryScrapeLock keeps one scrape running per database until release is called.
func (s *Store) TryScrapeLock(ctx context.Context) (release func() error, acquired bool, err error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("opening scrape lock connection: %w", err)
	}

	err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", scrapeLockID).Scan(&acquired)
	if err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()

		return nil, false, fmt.Errorf("acquiring scrape lock: %w", err)
	}

	if !acquired {
		return nil, false, conn.Close()
	}

	return func() error {
		defer conn.Close()

		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var unlocked bool

		err := conn.QueryRowContext(unlockCtx, "SELECT pg_advisory_unlock($1)", scrapeLockID).Scan(&unlocked)
		if err != nil {
			// Never return a potentially locked session to the connection pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })

			return fmt.Errorf("releasing scrape lock: %w", err)
		}

		if !unlocked {
			return errors.New("scrape lock was not held")
		}

		return nil
	}, true, nil
}
