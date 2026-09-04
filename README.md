# Blogbot

Telegram bot that scrapes blog posts from Japanese idol group websites (Nogizaka46, Sakurazaka46, Hinatazaka46), monitors Showroom live streams, and sends notifications to subscribed Telegram users. UI language is Traditional Chinese.



## Repository layout

```
.
├── cmd/blogbot/     Command-line entry points (scrape, webhook, migrate, …)
├── bot/             Telegram command, callback, and keyboard handling
├── scraper/         Per-group blog scrapers (Nogizaka46, Sakurazaka46, Hinatazaka46)
├── showroom/        Showroom live-stream monitoring
├── store/           PostgreSQL persistence
├── observability/   Structured logging, metrics, request IDs
├── config/          Configuration loading (file + environment)
└── webapp/          React frontend, served by the Go binary at /tg/blogbot
```

The server and the webapp live in one repository on purpose: the webapp is
served by the Go binary and both are picked up by a single service restart, so
they must be built, shipped, and restarted together.


## Configuration

Copy the example file and fill it in:

```bash
cp config.example.toml config.toml
$EDITOR config.toml
```

`config.toml` is gitignored — it holds live credentials and must never be
committed. `config.example.toml` documents every available key.

Every setting can also be supplied through the environment, which takes
precedence over the file. The variable name is `BLOGBOT_`, the section, and the
key, upper-cased and underscore-separated:

| Setting | Environment variable |
| --- | --- |
| `telegram.bot_token` | `BLOGBOT_TELEGRAM_BOT_TOKEN` |
| `telegram.log_chat_id` | `BLOGBOT_TELEGRAM_LOG_CHAT_ID` |
| `database.dsn` | `BLOGBOT_DATABASE_DSN` |
| `webhook.listen_addr` | `BLOGBOT_WEBHOOK_LISTEN_ADDR` |
| `webhook.webhook_url` | `BLOGBOT_WEBHOOK_WEBHOOK_URL` |
| `observability.log_level` | `BLOGBOT_OBSERVABILITY_LOG_LEVEL` |

The config file may be omitted entirely if the environment supplies
`bot_token` and `dsn`. If neither source provides them, startup fails with an
explicit error rather than falling back to defaults and connecting to the
wrong database.

## Deployment

Merges to `main` deploy automatically via GitHub Actions
(`.github/workflows/deploy.yml`). Only the compiled binary is shipped;
`config.toml` lives on the server and is managed out of band, so no
application credential is stored in GitHub.

To deploy manually from a workstation instead:

```bash
make deploy
```

## Build

```bash
# Requires Go 1.26+
go build -o blogbot ./cmd/blogbot/
```

Cross-compile for Linux from macOS:

```bash
GOOS=linux GOARCH=amd64 go build -o blogbot ./cmd/blogbot/
```

## Commands

```
blogbot scrape              # One-shot: scrape blogs, notify subscribers
blogbot showroom-nextlive   # One-shot: poll Showroom next_live API
blogbot showroom-live       # Long-running: WebSocket live detection
blogbot webhook             # Long-running: Telegram webhook HTTP server
blogbot migrate             # One-time: import Python-era JSON data
```

All commands accept `--config <path>` (default: `./config.toml`).

## Deploy to Oracle Cloud (Linux)

### 1. Provision the instance

- Create a **Compute instance** (ARM Ampere A1 free tier works) with Oracle Linux or Ubuntu
- In **Networking > VCN > Subnet > Security List**, add an ingress rule:

  | Field | Value |
  |-------|-------|
  | Stateful | Yes |
  | Source CIDR | `0.0.0.0/0` |
  | IP Protocol | TCP |
  | Source Port Range | All |
  | Destination Port Range | `443` |

- If using Oracle Linux, also open the OS firewall:

  ```bash
  sudo firewall-cmd --permanent --add-port=443/tcp
  sudo firewall-cmd --reload
  ```

### 2. Install Go

```bash
# Oracle Linux
sudo dnf install -y golang

# Ubuntu
sudo snap install go --classic
```

### 3. Install PostgreSQL

```bash
# Run the setup script (creates database and user)
./scripts/setup-postgres.sh

# Or manually:
sudo dnf install -y postgresql15-server postgresql15
sudo /usr/pgsql-15/bin/postgresql-15-setup initdb
sudo systemctl enable --now postgresql-15
sudo -u postgres createuser -P blogbot
sudo -u postgres createdb -O blogbot blogbot
```

### 4. Clone and configure

```bash
cd /home/opc
# Cloned as blogbot-go: the deployment directory, the systemd units, and the
# cron entries below all reference /home/opc/blogbot-go.
git clone https://github.com/btj93/blogbot.git blogbot-go
cd blogbot-go

# Edit config.toml with your bot token, PostgreSQL DSN, and other settings
vim config.toml
```

### 5. Deploy

```bash
# Build, install services, start everything
./scripts/start.sh

# Set up webhook TLS (self-signed cert) and register with Telegram
./scripts/setup-webhook.sh <YOUR_PUBLIC_IP>
```

### 6. (Optional) Import existing data

```bash
cd /home/opc/blogbot-go
./blogbot migrate --blog-json ./blog.json --showroom-json ./showroom.json --progress-txt ./blogProgress.txt
```

### 7. Verify

```bash
sudo systemctl status blogbot-webhook
sudo systemctl status blogbot-showroom-live
tail -f /home/opc/blogbot-go/logs/webhook.log
crontab -l | grep blogbot
```

### Migrating from SQLite

If upgrading from the SQLite version:

```bash
# 1. Stop services
./scripts/stop.sh

# 2. Set up PostgreSQL
./scripts/setup-postgres.sh

# 3. Pull latest code and rebuild
./scripts/update.sh

# 4. Migrate data
./scripts/migrate-sqlite-to-postgres.sh /home/opc/blogbot-go/blogbot.db

# 5. Start services
./scripts/start.sh
```

### What `scripts/start.sh` does

1. Checks PostgreSQL is running
2. Builds the Go binary in `/home/opc/blogbot-go`
3. Creates `logs/` and `backups/` subdirectories
4. Installs systemd services (`blogbot-webhook`, `blogbot-showroom-live`) with auto-restart
5. Installs cron jobs (blog scrape every 5min, showroom-nextlive every 7min, daily `pg_dump` backup)
6. Installs logrotate (14 days, compressed)
7. Starts/restarts all services

**Re-run `scripts/start.sh` at any time** to rebuild, redeploy, and restart. It is idempotent — existing database and backups are never overwritten.

### Stop everything

```bash
./scripts/stop.sh
```

Stops systemd services, removes cron jobs. Database and logs are preserved.

## Manual Deploy

<details>
<summary>Step-by-step instructions (click to expand)</summary>

### 1. Build and upload

```bash
# On your local machine (cross-compile for Linux)
GOOS=linux GOARCH=amd64 go build -o blogbot ./cmd/blogbot/
scp blogbot config.toml opc@your-server:/home/opc/blogbot-go/
```

### 2. Create directories

```bash
mkdir -p /home/opc/blogbot-go/logs /home/opc/blogbot-go/backups
```

### 3. Migrate existing data (one-time)

```bash
cd /home/opc/blogbot-go
./blogbot migrate \
  --blog-json ./blog.json \
  --showroom-json ./showroom.json \
  --progress-txt ./blogProgress.txt
```

### 4. Cron jobs

```cron
*/5 * * * * cd /home/opc/blogbot-go && ./blogbot scrape --config ./config.toml >> /home/opc/blogbot-go/logs/scrape.log 2>&1
*/7 * * * * cd /home/opc/blogbot-go && ./blogbot showroom-nextlive --config ./config.toml >> /home/opc/blogbot-go/logs/showroom-nextlive.log 2>&1
0 4 * * * pg_dump -U blogbot blogbot > /home/opc/blogbot-go/backups/blogbot-$(date +\%Y\%m\%d).sql && find /home/opc/blogbot-go/backups -name "*.sql" -mtime +30 -delete
```

### 5. Systemd services

Create `/etc/systemd/system/blogbot-webhook.service` and `/etc/systemd/system/blogbot-showroom-live.service` — see `scripts/start.sh` for the exact unit file contents.

```bash
sudo systemctl daemon-reload
sudo systemctl enable blogbot-webhook blogbot-showroom-live
sudo systemctl start blogbot-webhook blogbot-showroom-live
```

### 6. Log rotation

Create `/etc/logrotate.d/blogbot`:

```
/home/opc/blogbot-go/logs/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

### 7. Database backup

The daily cron job handles backups. To restore:

```bash
sudo systemctl stop blogbot-webhook blogbot-showroom-live
psql -U blogbot blogbot < /home/opc/blogbot-go/backups/blogbot-20260324.sql
sudo systemctl start blogbot-webhook blogbot-showroom-live
```

</details>

## Operations

```bash
# View recent logs
tail -f /var/log/blogbot/webhook.log
tail -f /var/log/blogbot/scrape.log

# Restart services
sudo systemctl restart blogbot-webhook
sudo systemctl restart blogbot-showroom-live

# Manual scrape
cd /home/opc/blogbot-go && ./blogbot scrape --config ./config.toml

# Query database
psql postgres://blogbot:blogbot@localhost:5432/blogbot -c "SELECT name, COUNT(*) FROM subscriptions JOIN members ON subscriptions.member_id = members.id GROUP BY name ORDER BY COUNT(*) DESC;"

# Database backup
pg_dump -U blogbot blogbot > backup.sql

# Database restore
psql -U blogbot blogbot < backup.sql
```

## Configuration

Config is loaded from TOML file. Every field can be overridden via environment variable with `BLOGBOT_` prefix:

```bash
BLOGBOT_TELEGRAM_BOT_TOKEN=xxx ./blogbot scrape
BLOGBOT_DATABASE_DSN="postgres://blogbot:pass@localhost:5432/blogbot?sslmode=disable" ./blogbot scrape
```

See `config.example.toml` for all options.
