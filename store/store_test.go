package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var testDSN string

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		slog.Error("could not construct pool", slog.Any("error", err))
		return
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15-alpine",
		Env: []string{
			"POSTGRES_USER=test",
			"POSTGRES_PASSWORD=test",
			"POSTGRES_DB=blogbot_test",
			"listen_addresses='*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		slog.Error("could not start resource", slog.Any("error", err))
		return
	}

	hostPort := resource.GetHostPort("5432/tcp")
	testDSN = fmt.Sprintf("postgres://test:test@localhost:%s/blogbot_test?sslmode=disable", hostPort)

	if err := pool.Retry(func() error {
		db, err := sql.Open("pgx", testDSN)
		if err != nil {
			return err
		}
		defer db.Close()

		return db.PingContext(context.Background())
	}); err != nil {
		slog.Error("could not connect to postgres", slog.Any("error", err))
		return
	}

	m.Run()

	if err := pool.Purge(resource); err != nil {
		slog.Error("could not purge resource", slog.Any("error", err))
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Clean all tables between tests (order matters for FK constraints)
		_, _ = s.db.Exec("TRUNCATE webapp_locks, showroom_rooms, blog_progress, subscriptions, members, groups CASCADE")
		_ = s.Close()
	})

	return s
}

func TestOpenAndMigrate(t *testing.T) {
	s := newTestStore(t)

	rows, err := s.db.QueryContext(
		context.Background(),
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var tables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}

		tables = append(tables, name)
	}

	want := []string{"blog_progress", "groups", "members", "showroom_rooms", "subscriptions", "webapp_locks"}
	if len(tables) != len(want) {
		t.Fatalf("got tables %v, want %v", tables, want)
	}

	for i, w := range want {
		if tables[i] != w {
			t.Errorf("table[%d]=%q, want %q", i, tables[i], w)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.migrate(); err != nil {
		t.Fatal(err)
	}
}
