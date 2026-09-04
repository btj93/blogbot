#!/usr/bin/env bash
set -euo pipefail

SQLITE_DB="${1:-}"
PG_DSN="${2:-postgres://blogbot:blogbot@localhost:5432/blogbot?sslmode=disable}"

if [ -z "$SQLITE_DB" ]; then
    echo "Usage: $0 <path-to-blogbot.db> [postgres-dsn]"
    echo "Example: $0 /home/opc/blogbot-go/blogbot.db"
    exit 1
fi

if [ ! -f "$SQLITE_DB" ]; then
    echo "Error: SQLite database not found: $SQLITE_DB"
    exit 1
fi

if ! command -v sqlite3 &>/dev/null; then
    echo "Error: sqlite3 is not installed. Install it first:"
    echo "  sudo dnf install -y sqlite"
    exit 1
fi

if ! command -v psql &>/dev/null; then
    echo "Error: psql is not installed."
    exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "=== SQLite → PostgreSQL Migration ==="

# 1. Export data from SQLite
echo "[1/3] Exporting data from SQLite..."

sqlite3 "$SQLITE_DB" <<'SQL' > "$TMPDIR/groups.csv"
.mode csv
.headers off
SELECT id, name, created_at, updated_at FROM groups ORDER BY id;
SQL

sqlite3 "$SQLITE_DB" <<'SQL' > "$TMPDIR/members.csv"
.mode csv
.headers off
SELECT id, group_id, name, generation, CASE WHEN disabled = 0 THEN 'f' ELSE 't' END, created_at, updated_at FROM members ORDER BY id;
SQL

sqlite3 "$SQLITE_DB" <<'SQL' > "$TMPDIR/subscriptions.csv"
.mode csv
.headers off
SELECT id, member_id, chat_id, created_at, updated_at FROM subscriptions ORDER BY id;
SQL

sqlite3 "$SQLITE_DB" <<'SQL' > "$TMPDIR/blog_progress.csv"
.mode csv
.headers off
SELECT id, url, created_at, updated_at FROM blog_progress ORDER BY id;
SQL

sqlite3 "$SQLITE_DB" <<'SQL' > "$TMPDIR/showroom_rooms.csv"
.mode csv
.headers off
SELECT id, member_id, room_id, url, next_live_epoch, next_live_text, created_at, updated_at FROM showroom_rooms ORDER BY id;
SQL

echo "  Exported to $TMPDIR/"

# 2. Import into PostgreSQL
echo "[2/3] Importing into PostgreSQL..."

# Truncate tables
psql "$PG_DSN" -v ON_ERROR_STOP=1 -c "TRUNCATE showroom_rooms, blog_progress, subscriptions, members, groups CASCADE;"

# Import CSVs (each \copy needs its own psql invocation)
psql "$PG_DSN" -c "\copy groups(id, name, created_at, updated_at) FROM '$TMPDIR/groups.csv' WITH (FORMAT csv)"
psql "$PG_DSN" -c "\copy members(id, group_id, name, generation, disabled, created_at, updated_at) FROM '$TMPDIR/members.csv' WITH (FORMAT csv, NULL '')"
psql "$PG_DSN" -c "\copy subscriptions(id, member_id, chat_id, created_at, updated_at) FROM '$TMPDIR/subscriptions.csv' WITH (FORMAT csv)"
psql "$PG_DSN" -c "\copy blog_progress(id, url, created_at, updated_at) FROM '$TMPDIR/blog_progress.csv' WITH (FORMAT csv)"
psql "$PG_DSN" -c "\copy showroom_rooms(id, member_id, room_id, url, next_live_epoch, next_live_text, created_at, updated_at) FROM '$TMPDIR/showroom_rooms.csv' WITH (FORMAT csv, NULL '')"

# Reset sequences so next INSERT gets MAX(id) + 1
psql "$PG_DSN" -v ON_ERROR_STOP=1 <<SQL
SELECT setval('groups_id_seq', COALESCE((SELECT MAX(id) FROM groups), 1));
SELECT setval('members_id_seq', COALESCE((SELECT MAX(id) FROM members), 1));
SELECT setval('subscriptions_id_seq', COALESCE((SELECT MAX(id) FROM subscriptions), 1));
SELECT setval('blog_progress_id_seq', COALESCE((SELECT MAX(id) FROM blog_progress), 1));
SELECT setval('showroom_rooms_id_seq', COALESCE((SELECT MAX(id) FROM showroom_rooms), 1));
SQL

# 3. Verify
echo "[3/3] Verifying..."

psql "$PG_DSN" -t <<SQL
SELECT 'groups: ' || COUNT(*) FROM groups
UNION ALL SELECT 'members: ' || COUNT(*) FROM members
UNION ALL SELECT 'subscriptions: ' || COUNT(*) FROM subscriptions
UNION ALL SELECT 'blog_progress: ' || COUNT(*) FROM blog_progress
UNION ALL SELECT 'showroom_rooms: ' || COUNT(*) FROM showroom_rooms;
SQL

echo ""
echo "=== Migration Complete ==="
