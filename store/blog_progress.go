package store

import (
	"context"
	"fmt"
)

// ClaimBlog atomically checks if a blog URL has been processed and claims it
// if not. Returns true if the blog was successfully claimed (i.e. not yet
// processed), false if it was already processed.
func (s *Store) ClaimBlog(ctx context.Context, url string) (bool, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO blog_progress (url) VALUES ($1) ON CONFLICT (url) DO NOTHING", url)
	if err != nil {
		return false, fmt.Errorf("claiming blog: %w", err)
	}

	n, _ := res.RowsAffected()

	return n > 0, nil
}
