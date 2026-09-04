#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/home/opc/blogbot-go"

echo "=== Blogbot Update ==="

# 1. Stop services
echo "[1/4] Stopping services..."
sudo systemctl stop blogbot-webhook 2>/dev/null && echo "  blogbot-webhook stopped" || echo "  blogbot-webhook was not running"
sudo systemctl stop blogbot-showroom-live 2>/dev/null && echo "  blogbot-showroom-live stopped" || echo "  blogbot-showroom-live was not running"

# 2. Pull latest code
echo "[2/4] Pulling latest code from main..."
cd "$INSTALL_DIR"
git fetch origin main
git reset --hard origin/main

# 3. Rebuild
echo "[3/4] Building binary..."
go build -o blogbot ./cmd/blogbot/
# Restore SELinux context so systemd can execute the binary
if command -v chcon &>/dev/null; then
    chcon -t bin_t "$INSTALL_DIR/blogbot"
fi

# 4. Restart services
echo "[4/4] Starting services..."
sudo systemctl start blogbot-webhook
sudo systemctl start blogbot-showroom-live

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
echo "Current version:"
git log --oneline -1
