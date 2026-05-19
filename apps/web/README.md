# Private Cloud Gateway — Web Dashboard

Vite + React 18 + TypeScript + Tailwind CSS.

## Development

```bash
cd apps/web && pnpm install && pnpm dev
# Opens http://localhost:5173 — proxies /api/* to Go Core at localhost:8080
```

See [docs/dev-local.md](../../docs/dev-local.md) for Go Core setup.

## The web app is the user-facing dashboard.

Responsibilities:

- login page
- dashboard
- app cards
- widgets
- server status
- backup status
- settings
- app management UI
