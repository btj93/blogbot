package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/btj93/blogbot/store"
)

var (
	blogJSONPath     string
	showroomJSONPath string
	progressTxtPath  string

	migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "One-time: import existing JSON files into database",
		RunE:  runMigrate,
	}
)

func init() {
	migrateCmd.Flags().StringVar(&blogJSONPath, "blog-json", "", "path to blog.json")
	migrateCmd.Flags().StringVar(&showroomJSONPath, "showroom-json", "", "path to showroom.json")
	migrateCmd.Flags().StringVar(&progressTxtPath, "progress-txt", "", "path to blogProgress.txt")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	ctx := context.Background()

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// 1. Import blog.json
	if blogJSONPath != "" {
		slog.InfoContext(ctx, "importing file", slog.String("path", blogJSONPath))

		data, err := os.ReadFile(filepath.Clean(blogJSONPath))
		if err != nil {
			return fmt.Errorf("reading blog.json: %w", err)
		}

		var blogData map[string]map[string][]string
		if err := json.Unmarshal(data, &blogData); err != nil {
			return fmt.Errorf("parsing blog.json: %w", err)
		}

		for groupName, members := range blogData {
			_, err := tx.ExecContext(ctx, "INSERT INTO groups (name) VALUES ($1) ON CONFLICT (name) DO NOTHING", groupName)
			if err != nil {
				return fmt.Errorf("inserting group %s: %w", groupName, err)
			}

			var groupID int64
			if err := tx.QueryRowContext(ctx, "SELECT id FROM groups WHERE name = $1", groupName).Scan(&groupID); err != nil {
				return fmt.Errorf("querying group id for %s: %w", groupName, err)
			}

			for memberName, chatIDs := range members {
				_, err := tx.ExecContext(ctx,
					"INSERT INTO members (group_id, name) VALUES ($1, $2) ON CONFLICT (group_id, name) DO NOTHING",
					groupID, memberName,
				)
				if err != nil {
					return fmt.Errorf("inserting member %s: %w", memberName, err)
				}

				var memberID int64
				if err := tx.QueryRowContext(ctx, "SELECT id FROM members WHERE group_id = $1 AND name = $2", groupID, memberName).
					Scan(&memberID); err != nil {
					return fmt.Errorf("querying member id for %s: %w", memberName, err)
				}

				for _, chatID := range chatIDs {
					_, err := tx.ExecContext(
						ctx,
						"INSERT INTO subscriptions (member_id, chat_id) VALUES ($1, $2) ON CONFLICT (member_id, chat_id) DO NOTHING",
						memberID,
						chatID,
					)
					if err != nil {
						return fmt.Errorf("inserting subscription: %w", err)
					}
				}

				slog.InfoContext(ctx, "imported member",
					slog.String("group", groupName),
					slog.String("member", memberName),
					slog.Int("subscriptions", len(chatIDs)),
				)
			}
		}

		slog.InfoContext(ctx, "imported blog.json", slog.Int("groups", len(blogData)))
	}

	// 2. Import showroom.json
	if showroomJSONPath != "" {
		slog.InfoContext(ctx, "importing file", slog.String("path", showroomJSONPath))

		data, err := os.ReadFile(filepath.Clean(showroomJSONPath))
		if err != nil {
			return fmt.Errorf("reading showroom.json: %w", err)
		}

		var showroomData map[string]map[string]json.RawMessage
		if err := json.Unmarshal(data, &showroomData); err != nil {
			return fmt.Errorf("parsing showroom.json: %w", err)
		}

		for groupName, members := range showroomData {
			_, err := tx.ExecContext(ctx, "INSERT INTO groups (name) VALUES ($1) ON CONFLICT (name) DO NOTHING", groupName)
			if err != nil {
				return fmt.Errorf("inserting group %s: %w", groupName, err)
			}

			var groupID int64
			if err := tx.QueryRowContext(ctx, "SELECT id FROM groups WHERE name = $1", groupName).Scan(&groupID); err != nil {
				return fmt.Errorf("querying group id for %s: %w", groupName, err)
			}

			for memberName, raw := range members {
				_, err := tx.ExecContext(ctx,
					"INSERT INTO members (group_id, name) VALUES ($1, $2) ON CONFLICT (group_id, name) DO NOTHING",
					groupID, memberName,
				)
				if err != nil {
					return fmt.Errorf("inserting member %s: %w", memberName, err)
				}

				var memberID int64
				if err := tx.QueryRowContext(ctx, "SELECT id FROM members WHERE group_id = $1 AND name = $2", groupID, memberName).
					Scan(&memberID); err != nil {
					return fmt.Errorf("querying member id for %s: %w", memberName, err)
				}

				var entry struct {
					ID       string           `json:"ID"`
					URL      string           `json:"URL"`
					NextLive *json.RawMessage `json:"nextLive"`
				}
				if err := json.Unmarshal(raw, &entry); err != nil {
					slog.ErrorContext(
						ctx,
						"failed to parse showroom entry",
						slog.String("member", memberName),
						slog.Any("error", err),
					)

					continue
				}

				var (
					epoch *int64
					text  *string
				)

				if entry.NextLive != nil && string(*entry.NextLive) != "null" {
					var nl struct {
						Epoch *int64 `json:"epoch"`
						Text  string `json:"text"`
					}
					if err := json.Unmarshal(*entry.NextLive, &nl); err == nil {
						epoch = nl.Epoch
						if nl.Text != "" {
							text = &nl.Text
						}
					}
				}

				_, err = tx.ExecContext(ctx,
					`INSERT INTO showroom_rooms (member_id, room_id, url, next_live_epoch, next_live_text)
					 VALUES ($1, $2, $3, $4, $5)
					 ON CONFLICT(member_id) DO UPDATE SET room_id = excluded.room_id, url = excluded.url,
					 next_live_epoch = excluded.next_live_epoch, next_live_text = excluded.next_live_text,
					 updated_at = NOW()`,
					memberID, entry.ID, entry.URL, epoch, text,
				)
				if err != nil {
					return fmt.Errorf("inserting showroom room: %w", err)
				}

				slog.InfoContext(ctx, "imported showroom room",
					slog.String("group", groupName),
					slog.String("member", memberName),
					slog.String("room_id", entry.ID),
				)
			}
		}

		slog.InfoContext(ctx, "imported showroom.json")
	}

	// 3. Import blogProgress.txt
	if progressTxtPath != "" {
		slog.InfoContext(ctx, "importing file", slog.String("path", progressTxtPath))

		f, err := os.Open(filepath.Clean(progressTxtPath))
		if err != nil {
			return fmt.Errorf("opening progress file: %w", err)
		}
		defer f.Close()

		count := 0

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url == "" {
				continue
			}

			_, err := tx.ExecContext(ctx, "INSERT INTO blog_progress (url) VALUES ($1) ON CONFLICT (url) DO NOTHING", url)
			if err != nil {
				return fmt.Errorf("inserting blog progress: %w", err)
			}

			slog.InfoContext(ctx, "imported blog progress url", slog.String("url", url))

			count++
		}

		slog.InfoContext(ctx, "imported blog progress", slog.Int("count", count))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	slog.InfoContext(ctx, "migration complete")

	return nil
}
