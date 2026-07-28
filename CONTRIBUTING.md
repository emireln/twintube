# Contributing to TwinTube

Thanks for helping improve TwinTube. This project is a Go backend + vanilla JS frontend for synchronized watch rooms.

## Development setup

1. Install **Go 1.21+**.
2. Clone and run:

```bash
git clone https://github.com/emireln/twintube.git
cd twintube
go run .
```

Open [http://localhost:8080](http://localhost:8080).

Optional Docker (app + PostgreSQL):

```bash
cp .env.example .env
# set a real JWT_SECRET
docker compose up -d --build
```

Windows desktop client: see [`desktop/README.md`](desktop/README.md).

## Project layout

| Path | Role |
|------|------|
| `main.go` | HTTP + WebSocket server entrypoint |
| `internal/` | Auth, rooms, DB, security, utils |
| `static/` | Landing, room UI, CSS, JS modules |
| `desktop/` | Electron Windows client |
| `deploy/` | Production Compose / Caddy / coturn |
| `scripts/` | Version bump, local start/stop, VPS helpers |
| `AGENTS.md` | Architecture + conventions (for humans and AI agents) |

## Pull requests

- Keep changes focused; match existing style in the files you touch.
- Prefer helpers on `Room` / `RoomManager` for state; guard with `sync.RWMutex`.
- Frontend: follow Material 3 tokens in `static/css/style.css`; keep i18n keys in EN + PT (`static/js/i18n.js`).
- Do not commit `.env`, secrets, binaries, or `node_modules` / `desktop/dist`.
- Describe **why** the change is needed in the PR body; include a short test plan.

## Reporting issues

Include: TwinTube version (`VERSION`), browser/OS, whether Docker or `go run`, and steps to reproduce. For sync bugs, note video source (YouTube / local / etc.) and approximate drift.

## Code of conduct

Be respectful. Harassment or abuse is not welcome. Maintainers may close issues/PRs that violate this.
