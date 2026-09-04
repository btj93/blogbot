#!/usr/bin/env bash
set -euo pipefail

echo "=== Stopping Blogbot ==="

# 1. Stop and disable systemd services
echo "[1/2] Stopping services..."
sudo systemctl stop blogbot-webhook 2>/dev/null && echo "  blogbot-webhook stopped" || echo "  blogbot-webhook was not running"
sudo systemctl stop blogbot-showroom-live 2>/dev/null && echo "  blogbot-showroom-live stopped" || echo "  blogbot-showroom-live was not running"
sudo systemctl disable blogbot-webhook blogbot-showroom-live 2>/dev/null || true

# 2. Remove cron jobs
echo "[2/2] Removing cron jobs..."
(crontab -l 2>/dev/null | grep -v 'blogbot-go' || true) | crontab -
echo "  Cron entries removed"

echo ""
echo "=== Blogbot stopped ==="
echo "Database and logs are preserved."
echo "Run scripts/start.sh to start again."
