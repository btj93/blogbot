#!/usr/bin/env bash
set -euo pipefail

# Sets up TLS via Let's Encrypt and registers the Telegram webhook.
#
# Prerequisites:
#   - Domain DNS A record pointing to this server
#   - Port 80 open in OCI security list (for ACME HTTP challenge)
#   - Port 443 open in OCI security list (for webhook + webapp)
#
# Usage: ./scripts/setup-webhook.sh <DOMAIN>
# Example: ./scripts/setup-webhook.sh example.com

INSTALL_DIR="/home/opc/blogbot-go"

DOMAIN="${1:-}"
if [ -z "$DOMAIN" ]; then
    echo "Usage: $0 <DOMAIN>"
    echo "Example: $0 example.com"
    exit 1
fi

CERT_PATH="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
KEY_PATH="/etc/letsencrypt/live/$DOMAIN/privkey.pem"

# Read bot token from config
CONFIG="$INSTALL_DIR/config.toml"
if [ ! -f "$CONFIG" ]; then
    CONFIG="$(cd "$(dirname "$0")/.." && pwd)/config.toml"
fi
BOT_TOKEN=$(grep 'bot_token' "$CONFIG" | head -1 | sed 's/.*"\(.*\)".*/\1/')

if [ -z "$BOT_TOKEN" ]; then
    echo "Error: Could not read bot_token from config.toml"
    exit 1
fi

WEBHOOK_URL="https://$DOMAIN/$BOT_TOKEN"

echo "=== Webhook Setup ==="
echo "Domain:      $DOMAIN"
echo "Webhook URL: $WEBHOOK_URL"
echo ""

# 1. Install certbot if not present
if ! command -v certbot &>/dev/null; then
    echo "[1/7] Installing certbot..."
    # Oracle Linux needs EPEL for certbot
    sudo dnf install -y oracle-epel-release-el8 2>/dev/null \
        || sudo dnf install -y epel-release 2>/dev/null \
        || true
    sudo dnf install -y certbot
else
    echo "[1/7] certbot already installed."
fi

# 2. Obtain Let's Encrypt certificate
if [ -f "$CERT_PATH" ] && [ -f "$KEY_PATH" ]; then
    echo "[2/7] Certificate already exists at $CERT_PATH, skipping."
    echo "       Run 'sudo certbot renew' to renew."
else
    echo "[2/7] Obtaining Let's Encrypt certificate..."
    echo "       Temporarily stopping blogbot-webhook to free port if needed..."
    sudo systemctl stop blogbot-webhook 2>/dev/null || true

    sudo certbot certonly --standalone -d "$DOMAIN" --non-interactive --agree-tos --register-unsafely-without-email

    echo "       Cert: $CERT_PATH"
    echo "       Key:  $KEY_PATH"
fi

# 3. Update config.toml with cert paths
echo "[3/7] Updating config.toml with cert paths..."
sed -i "s|^tls_cert = .*|tls_cert = \"$CERT_PATH\"|" "$CONFIG"
sed -i "s|^tls_key = .*|tls_key = \"$KEY_PATH\"|" "$CONFIG"
sed -i "s|^webhook_url = .*|webhook_url = \"$WEBHOOK_URL\"|" "$CONFIG"
echo "       Done."

# Make certs readable by the blogbot service (runs as opc).
# The symlinks in live/ point to files in archive/, so both need to be accessible.
sudo chmod 755 /etc/letsencrypt/live /etc/letsencrypt/archive
sudo chmod 755 "/etc/letsencrypt/live/$DOMAIN" "/etc/letsencrypt/archive/$DOMAIN"
sudo chmod 644 /etc/letsencrypt/archive/"$DOMAIN"/*.pem

# 4. Install certbot renewal deploy hook
echo "[4/7] Installing certbot renewal deploy hook..."
# certbot renews unattended (~30 days before expiry) but does NOT restart us.
# Two things then break unless we intervene:
#   1. certbot writes fresh key/cert files into archive/ with root-only perms,
#      so the service (runs as opc) can no longer read the renewed privkey.
#   2. Go's ListenAndServeTLS loads the cert into memory once at startup and
#      never re-reads it, so the running process keeps serving the OLD cert.
# This hook fixes both perms and bounces the service after every renewal.
HOOK="/etc/letsencrypt/renewal-hooks/deploy/blogbot-webhook.sh"
sudo mkdir -p "$(dirname "$HOOK")"
sudo tee "$HOOK" >/dev/null <<EOF
#!/bin/sh
# Installed by setup-webhook.sh. Runs after certbot renews a certificate.
set -e
chmod 644 /etc/letsencrypt/archive/$DOMAIN/*.pem
systemctl restart blogbot-webhook
EOF
sudo chmod +x "$HOOK"
echo "       Hook: $HOOK"

# 5. Allow binding to port 443 without root
echo "[5/7] Setting cap_net_bind_service on binary..."
sudo setcap cap_net_bind_service=+ep "$INSTALL_DIR/blogbot"
echo "       Done."

# Start/restart the webhook service
echo "       Starting blogbot-webhook..."
sudo systemctl start blogbot-webhook
sleep 2

# 6. Health check
echo "[6/7] Checking webhook server health..."
HEALTH=$(curl -sk --connect-timeout 5 "https://localhost/health" 2>/dev/null || true)
if [ "$HEALTH" = "ok" ]; then
    echo "       Server is alive."
else
    echo "       ERROR: Webhook server is not responding on https://localhost/health"
    echo "       Check: sudo systemctl status blogbot-webhook"
    exit 1
fi

# 7. Register webhook with Telegram (trusted CA, no cert upload needed)
echo "[7/7] Registering webhook with Telegram..."
RESPONSE=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/setWebhook?url=$WEBHOOK_URL")
echo "       Response: $RESPONSE"

# Verify
echo ""
echo "       Verifying..."
INFO=$(curl -s "https://api.telegram.org/bot$BOT_TOKEN/getWebhookInfo")
echo "       $INFO"

echo ""
echo "=== Done ==="
echo "Webhook registered at $WEBHOOK_URL"
echo ""
echo "Certbot auto-renews via systemd timer. Check with: sudo systemctl list-timers certbot*"
echo "Renewal auto-restarts the webhook via the deploy hook: $HOOK"
echo ""
echo "Make sure Oracle Cloud security list allows inbound TCP on ports 80 (ACME) and 443 (webhook + webapp)."
