#!/usr/bin/env bash
set -euo pipefail

# Builds the Go binary and the React webapp, then deploys both to the OCI
# server in a single pass.
#
# The two halves are deployed together on purpose: the webapp is served by the
# Go binary, and both are picked up by the same systemd restart. Deploying them
# independently risks restarting the service while the other half is mid-copy.
#
# Usage: ./scripts/deploy.sh
#
# Prerequisites:
#   - Go and bun installed locally
#   - SSH access to the deployment host
#
# Deployment targets come from the environment so that this repository does not
# publish the host, login, or key filename. Put them in a local .envrc, your
# shell profile, or pass them inline:
#
#   DEPLOY_HOST=user@example.com DEPLOY_DIR=/srv/blogbot ./scripts/deploy.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REMOTE_HOST="${DEPLOY_HOST:?set DEPLOY_HOST, e.g. user@example.com}"
REMOTE_DIR="${DEPLOY_DIR:-/home/opc/blogbot-go}"
SSH_KEY="${DEPLOY_SSH_KEY:?set DEPLOY_SSH_KEY to your private key path}"
SSH="ssh -i $SSH_KEY $REMOTE_HOST"

echo "=== Blogbot Deploy ==="
echo ""

# config.toml is deliberately untracked (it holds live credentials), so a fresh
# clone will not have one. Fail here rather than deploying a binary against a
# stale or absent server config.
if [ ! -f "$PROJECT_DIR/config.toml" ]; then
    echo "Error: $PROJECT_DIR/config.toml not found." >&2
    echo "       Copy config.example.toml to config.toml and fill in your values." >&2
    exit 1
fi

echo "[1/5] Building binary (linux/arm64)..."
cd "$PROJECT_DIR"
GOOS=linux GOARCH=arm64 go build -o blogbot ./cmd/blogbot/
echo "       Build complete."

echo "[2/5] Building webapp..."
cd "$PROJECT_DIR/webapp"
bun install --frozen-lockfile
bun run build
cd "$PROJECT_DIR"
echo "       Build complete."

echo "[3/5] Stopping services..."
$SSH "sudo systemctl stop blogbot-webhook 2>/dev/null || true; sudo systemctl stop blogbot-showroom-live 2>/dev/null || true"
echo "       Services stopped."

echo "[4/5] Uploading binary, config, and webapp..."
rsync -az -e "ssh -i $SSH_KEY" "$PROJECT_DIR/blogbot" "$PROJECT_DIR/config.toml" "$REMOTE_HOST:$REMOTE_DIR/"
$SSH "mkdir -p $REMOTE_DIR/webapp"
rsync -az --delete -e "ssh -i $SSH_KEY" "$PROJECT_DIR/webapp/build/" "$REMOTE_HOST:$REMOTE_DIR/webapp/"
$SSH "chcon -t bin_t $REMOTE_DIR/blogbot 2>/dev/null || true; sudo setcap cap_net_bind_service=+ep $REMOTE_DIR/blogbot"
echo "       Upload complete."

echo "[5/5] Starting services..."
$SSH "sudo systemctl start blogbot-webhook; sudo systemctl start blogbot-showroom-live"

OK=true
for SVC in blogbot-webhook blogbot-showroom-live; do
    ACTIVE=false
    for _ in $(seq 1 10); do
        if $SSH "sudo systemctl is-active $SVC --quiet"; then
            echo "       $SVC: active"; ACTIVE=true; break
        fi
        sleep 1
    done
    if ! $ACTIVE; then echo "       $SVC: FAILED (not active after 10s)"; OK=false; fi
done

rm -f "$PROJECT_DIR/blogbot"

echo ""
if $OK; then
    echo "=== Deploy complete ==="
else
    echo "=== Deploy finished with errors — check logs on server ==="
    exit 1
fi
