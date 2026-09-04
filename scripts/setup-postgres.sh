#!/usr/bin/env bash
set -euo pipefail

DB_NAME="blogbot"
DB_USER="blogbot"
DB_PASS="${1:-blogbot}"

echo "=== PostgreSQL Setup ==="

# 1. Install PostgreSQL 15
echo "[1/4] Installing PostgreSQL 15..."
if command -v dnf &>/dev/null; then
    # Oracle Linux / RHEL: install PGDG repo for PostgreSQL 15
    if ! rpm -q pgdg-redhat-repo &>/dev/null; then
        sudo dnf install -y "https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %{rhel})-$(uname -m)/pgdg-redhat-repo-latest.noarch.rpm"
        sudo dnf -qy module disable postgresql 2>/dev/null || true
    fi
    sudo dnf install -y postgresql15-server postgresql15
    PG_SETUP="/usr/pgsql-15/bin/postgresql-15-setup"
    PG_SERVICE="postgresql-15"
    if [ ! -f /var/lib/pgsql/15/data/PG_VERSION ]; then
        sudo "$PG_SETUP" initdb
    fi
    sudo systemctl enable "$PG_SERVICE"
    sudo systemctl start "$PG_SERVICE"
elif command -v apt-get &>/dev/null; then
    sudo apt-get update
    sudo apt-get install -y postgresql postgresql-contrib
    PG_SERVICE="postgresql"
    sudo systemctl enable "$PG_SERVICE"
    sudo systemctl start "$PG_SERVICE"
else
    echo "Unsupported package manager. Install PostgreSQL manually."
    exit 1
fi

# 2. Wait for PostgreSQL to be ready
echo "[2/4] Waiting for PostgreSQL..."
for i in $(seq 1 10); do
    if sudo -u postgres pg_isready -q 2>/dev/null; then
        echo "  PostgreSQL is ready."
        break
    fi
    sleep 1
done

# 3. Create user and database
echo "[3/4] Creating database and user..."
sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${DB_USER}') THEN
        CREATE ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASS}';
    END IF;
END
\$\$;

SELECT 'CREATE DATABASE ${DB_NAME} OWNER ${DB_USER}'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DB_NAME}')\gexec
SQL

# 4. Configure pg_hba.conf for password auth
echo "[4/4] Configuring authentication..."
PG_HBA=$(sudo -u postgres psql -t -c "SHOW hba_file" | xargs)
if ! sudo grep -q "host.*${DB_NAME}.*${DB_USER}" "$PG_HBA"; then
    # Add before the first "host" line
    sudo sed -i "/^host/i host    ${DB_NAME}    ${DB_USER}    127.0.0.1/32    scram-sha-256" "$PG_HBA"
    sudo sed -i "/^host/i host    ${DB_NAME}    ${DB_USER}    ::1/128         scram-sha-256" "$PG_HBA"
    sudo systemctl reload "$PG_SERVICE"
fi

echo ""
echo "=== PostgreSQL Ready ==="
echo ""
echo "  Database: ${DB_NAME}"
echo "  User:     ${DB_USER}"
echo "  Password: ${DB_PASS}"
echo "  DSN:      postgres://${DB_USER}:${DB_PASS}@localhost:5432/${DB_NAME}?sslmode=disable"
echo ""
echo "Update config.toml with the DSN above."
