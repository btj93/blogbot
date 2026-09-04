package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/btj93/blogbot/model"
)

func (s *Store) GetOrCreateGroup(ctx context.Context, name string) (*model.Group, error) {
	_, err := s.db.ExecContext(ctx, "INSERT INTO groups (name) VALUES ($1) ON CONFLICT (name) DO NOTHING", name)
	if err != nil {
		return nil, fmt.Errorf("inserting group: %w", err)
	}

	return s.GetGroupByName(ctx, name)
}

func (s *Store) GetGroupByName(ctx context.Context, name string) (*model.Group, error) {
	var g model.Group

	err := s.db.QueryRowContext(ctx, "SELECT id, name, created_at, updated_at FROM groups WHERE name = $1", name).
		Scan(&g.ID, &g.Name, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting group: %w", err)
	}

	return &g, nil
}

func (s *Store) ListGroups(ctx context.Context) ([]model.Group, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, created_at, updated_at FROM groups ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	defer rows.Close()

	var groups []model.Group

	for rows.Next() {
		var g model.Group

		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}

		groups = append(groups, g)
	}

	return groups, rows.Err()
}

func (s *Store) GetGroupByID(ctx context.Context, id int64) (*model.Group, error) {
	var g model.Group

	err := s.db.QueryRowContext(ctx, "SELECT id, name, created_at, updated_at FROM groups WHERE id = $1", id).
		Scan(&g.ID, &g.Name, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found: %d", id)
	}

	if err != nil {
		return nil, fmt.Errorf("getting group: %w", err)
	}

	return &g, nil
}
