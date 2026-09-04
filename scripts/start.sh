#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/home/opc/blogbot-go"
LOG_DIR="/var/log/blogbot"
BACKUP_DIR="$INSTALL_DIR/backups"
USER="$(whoami)"

echo "=== Blogbot Setup & Start ==="

# 1. Build
echo "[1/7] Building binary..."
cd "$INSTALL_DIR"
go build -o blogbot ./cmd/blogbot/
# Restore SELinux context so systemd can execute the binary
if command -v chcon &>/dev/null; then
    chcon -t bin_t "$INSTALL_DIR/blogbot"
fi
# Allow binding to port 443 without root
sudo setcap cap_net_bind_service=+ep "$INSTALL_DIR/blogbot"

# 2. Create directories
echo "[2/7] Creating directories..."
sudo mkdir -p "$LOG_DIR"
sudo chown "$USER":"$USER" "$LOG_DIR"
mkdir -p "$BACKUP_DIR"
# Allow systemd to traverse home dir to reach the binary
chmod 711 /home/opc

# Check PostgreSQL is running
echo "[3/7] Checking PostgreSQL..."
if command -v pg_isready &>/dev/null; then
    if pg_isready -q 2>/dev/null || sudo -u postgres pg_isready -q 2>/dev/null; then
        echo "  PostgreSQL is running."
    else
        echo "  ERROR: PostgreSQL is not running."
        echo "  Run: sudo systemctl start postgresql-15"
        exit 1
    fi
else
    echo "  WARNING: pg_isready not found, skipping PostgreSQL check."
fi

# Set up .pgpass from config.toml DSN for pg_dump
DSN=$(grep -oP 'dsn\s*=\s*"\K[^"]+' "$INSTALL_DIR/config.toml")
if [ -n "$DSN" ]; then
    DB_USER=$(echo "$DSN" | sed -n 's|.*://\([^:]*\):.*|\1|p')
    DB_PASS=$(echo "$DSN" | sed -n 's|.*://[^:]*:\([^@]*\)@.*|\1|p')
    DB_HOST=$(echo "$DSN" | sed -n 's|.*@\([^:]*\):.*|\1|p')
    DB_PORT=$(echo "$DSN" | sed -n 's|.*@[^:]*:\([^/]*\)/.*|\1|p')
    DB_NAME=$(echo "$DSN" | sed -n 's|.*@[^/]*/\([^?]*\).*|\1|p')
    PGPASS_LINE="${DB_HOST}:${DB_PORT}:${DB_NAME}:${DB_USER}:${DB_PASS}"
    PGPASS_FILE="$HOME/.pgpass"
    if [ ! -f "$PGPASS_FILE" ] || ! grep -qF "$PGPASS_LINE" "$PGPASS_FILE"; then
        echo "$PGPASS_LINE" >> "$PGPASS_FILE"
        chmod 600 "$PGPASS_FILE"
    fi
fi

# 4. Install systemd services
echo "[4/7] Installing systemd services..."

sudo tee /etc/systemd/system/blogbot-webhook.service >/dev/null <<EOF
[Unit]
Description=Blogbot Telegram Webhook
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/blogbot webhook --config $INSTALL_DIR/config.toml
Restart=always
RestartSec=5
StandardOutput=append:$LOG_DIR/webhook.log
StandardError=append:$LOG_DIR/webhook.log

[Install]
WantedBy=multi-user.target
EOF

sudo tee /etc/systemd/system/blogbot-showroom-live.service >/dev/null <<EOF
[Unit]
Description=Blogbot Showroom Live Monitor
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/blogbot showroom-live --config $INSTALL_DIR/config.toml
Restart=always
RestartSec=10
StandardOutput=append:$LOG_DIR/showroom-live.log
StandardError=append:$LOG_DIR/showroom-live.log

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload

# 5. Install cron jobs
echo "[5/7] Installing cron jobs..."

# Remove old blogbot cron entries, then add fresh ones
(
  crontab -l 2>/dev/null | grep -v 'blogbot-go' || true
  cat <<EOF
*/5 * * * * cd $INSTALL_DIR && ./blogbot scrape --config ./config.toml >> $LOG_DIR/scrape.log 2>&1
*/7 * * * * cd $INSTALL_DIR && ./blogbot showroom-nextlive --config ./config.toml >> $LOG_DIR/showroom-nextlive.log 2>&1
0 4 * * * pg_dump -U blogbot -h localhost blogbot | gzip > $BACKUP_DIR/blogbot-\$(date +\%Y\%m\%d).sql.gz && find $BACKUP_DIR -name "*.sql.gz" -mtime +30 -delete
EOF
) | crontab -

# 6. Install logrotate
echo "[6/7] Installing logrotate..."
sudo tee /etc/logrotate.d/blogbot >/dev/null <<EOF
$LOG_DIR/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
EOF

# 7. Start services
echo "[7/7] Starting services..."
sudo systemctl enable blogbot-webhook blogbot-showroom-live
sudo systemctl restart blogbot-webhook
sudo systemctl restart blogbot-showroom-live

echo ""
echo "=== Waiting for services to start ==="
echo ""

check_service() {
  local name="$1"
  local max_attempts=10
  for i in $(seq 1 $max_attempts); do
    if sudo systemctl is-active "$name" --quiet; then
      echo "  $name: active"
      return 0
    fi
    sleep 1
  done
  echo "  $name: FAILED (not active after ${max_attempts}s)"
  echo "  Check logs: journalctl -u $name -n 20"
  return 1
}

echo "Services:"
check_service blogbot-webhook
check_service blogbot-showroom-live
echo ""
echo "Cron jobs:"
crontab -l 2>/dev/null | grep blogbot | while read -r line; do echo "  $line"; done
echo ""
echo "Logs:    $LOG_DIR/"
echo "Backups: $BACKUP_DIR/"
echo "DB:      PostgreSQL (blogbot)"
echo ""
if [ ! -f "$INSTALL_DIR/cert.pem" ]; then
  echo "NOTE: Webhook TLS not set up yet. Run:"
  echo "  ./scripts/setup-webhook.sh <YOUR_PUBLIC_IP>"
fi
