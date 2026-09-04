package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/btj93/blogbot/model"
)

func (s *Store) GetOrCreateMember(
	ctx context.Context,
	groupID int64,
	name string,
	generation *int,
	disabled bool,
) (*model.Member, error) {
	_, err := s.db.ExecContext(
		ctx,
		"INSERT INTO members (group_id, name, generation, disabled) VALUES ($1, $2, $3, $4) ON CONFLICT (group_id, name) DO NOTHING",
		groupID,
		name,
		generation,
		disabled,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting member: %w", err)
	}

	return s.GetMemberByGroupAndName(ctx, groupID, name)
}

func (s *Store) GetMemberByGroupAndName(ctx context.Context, groupID int64, name string) (*model.Member, error) {
	var (
		m   model.Member
		gen sql.NullInt64
	)

	err := s.db.QueryRowContext(
		ctx,
		"SELECT id, group_id, name, generation, disabled, created_at, updated_at FROM members WHERE group_id = $1 AND name = $2",
		groupID,
		name,
	).Scan(&m.ID, &m.GroupID, &m.Name, &gen, &m.Disabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting member: %w", err)
	}

	if gen.Valid {
		v := int(gen.Int64)
		m.Generation = &v
	}

	return &m, nil
}

func (s *Store) ListMembersByGroup(ctx context.Context, groupID int64) ([]model.Member, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT id, group_id, name, generation, disabled, created_at, updated_at FROM members WHERE group_id = $1 ORDER BY generation NULLS LAST, name",
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}
	defer rows.Close()

	return scanMembers(rows)
}

func (s *Store) ListEnabledMembersByGroupAndGeneration(
	ctx context.Context,
	groupID int64,
	generation *int,
) ([]model.Member, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if generation == nil {
		rows, err = s.db.QueryContext(
			ctx,
			"SELECT id, group_id, name, generation, disabled, created_at, updated_at FROM members WHERE group_id = $1 AND generation IS NULL AND disabled = FALSE ORDER BY name",
			groupID,
		)
	} else {
		rows, err = s.db.QueryContext(
			ctx,
			"SELECT id, group_id, name, generation, disabled, created_at, updated_at FROM members WHERE group_id = $1 AND generation = $2 AND disabled = FALSE ORDER BY name",
			groupID,
			*generation,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("listing members by generation: %w", err)
	}

	defer rows.Close()

	return scanMembers(rows)
}

func (s *Store) ListGenerationsForGroup(ctx context.Context, groupID int64) ([]*int, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT generation FROM members WHERE group_id = $1 AND disabled = FALSE ORDER BY generation NULLS LAST",
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing generations: %w", err)
	}
	defer rows.Close()

	var gens []*int

	for rows.Next() {
		var gen sql.NullInt64
		if err := rows.Scan(&gen); err != nil {
			return nil, err
		}

		if gen.Valid {
			v := int(gen.Int64)
			gens = append(gens, &v)
		} else {
			gens = append(gens, nil)
		}
	}

	return gens, rows.Err()
}

func (s *Store) GetMemberByID(ctx context.Context, id int64) (*model.Member, error) {
	var (
		m   model.Member
		gen sql.NullInt64
	)

	err := s.db.QueryRowContext(ctx,
		"SELECT id, group_id, name, generation, disabled, created_at, updated_at FROM members WHERE id = $1",
		id,
	).Scan(&m.ID, &m.GroupID, &m.Name, &gen, &m.Disabled, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("member not found: %d", id)
	}

	if err != nil {
		return nil, fmt.Errorf("getting member: %w", err)
	}

	if gen.Valid {
		v := int(gen.Int64)
		m.Generation = &v
	}

	return &m, nil
}

func scanMembers(rows *sql.Rows) ([]model.Member, error) {
	var members []model.Member

	for rows.Next() {
		var (
			m   model.Member
			gen sql.NullInt64
		)

		if err := rows.Scan(&m.ID, &m.GroupID, &m.Name, &gen, &m.Disabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}

		if gen.Valid {
			v := int(gen.Int64)
			m.Generation = &v
		}

		members = append(members, m)
	}

	return members, rows.Err()
}
