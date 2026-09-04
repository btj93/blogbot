# Blogbot WebApp

The React frontend for [Blogbot](../README.md), opened as a Telegram Web App
from the bot's inline keyboards. It is built as a static bundle and served by
the Go binary under `/tg/blogbot`.

## Development

```bash
bun install
bun run start      # dev server on http://localhost:8080
bun run build      # production bundle into build/
```

`PUBLIC_URL=/tg/blogbot` in `.env.production` fixes the deployed base path; the
bundle will not resolve its assets if that value is changed without also
changing where the Go server mounts it.

## Deployment

The webapp is not deployed on its own. `scripts/deploy.sh` at the repository
root — and the `Deploy` workflow on merges to `main` — build and ship it
alongside the server binary, then restart the service once for both.
