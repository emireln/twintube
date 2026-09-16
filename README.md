<p align="center">
  <img src="static/logo.svg" alt="TwinTube Logo" width="140" height="140">
</p>

<h1 align="center">TwinTube</h1>

<p align="center">
  <b>Real-Time Synchronized Video Watching Platform</b><br>
  Inspired by SyncTube &amp; Google Material Design 3 • Built with Go, WebSockets &amp; PostgreSQL
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/Database-PostgreSQL%20%7C%20SQLite-4169E1?style=flat&logo=postgresql&logoColor=white" alt="Database">
  <img src="https://img.shields.io/badge/WebSockets-Gorilla-FF69B4?style=flat" alt="WebSockets">
  <img src="https://img.shields.io/badge/Styling-Material%20Design%203-757575?style=flat&logo=google" alt="Material Design 3">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue.svg" alt="License">
  <a href="https://github.com/emireln/twintube"><img src="https://img.shields.io/github/stars/emireln/twintube?style=flat&logo=github" alt="GitHub stars"></a>
  <a href="https://buymeacoffee.com/emireln" target="_blank"><img src="https://img.shields.io/badge/Buy%20Me%20A%20Coffee-Donate-FFDD00?style=flat&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee"></a>
</p>

---

## Features

- **Watch together** — Server-authoritative sync over WebSockets with drift correction (&gt;1.5s auto-seek).
- **Multi-source media** — YouTube, Vimeo, Twitch, direct links, and **Local Night** (files stay on each device; only fingerprints + playback sync).
- **Collaborative queue** — Add, reorder, remove, and auto-advance; host can lock down who can edit.
- **Open-by-default rooms** — Hosts/co-hosts control permissions for playback, queue, moments, and jumps.
- **Moments** — Save timestamps, approve as host/co-host, jump everyone to a highlight.
- **Live chat** — Timestamped messages, system alerts, and profile avatars.
- **Fast GIF reactions** — Compact player dock with allowlisted WebP reactions.
- **Push-to-talk voice** — Optional WebRTC mesh (STUN + optional self-hosted coturn).
- **Auth & guests** — JWT + bcrypt accounts, or join as guest; EN/PT UI.
- **Mobile room UX** — Slim top bar, left drawer for room tools, bottom tabs for Chat / Queue / Viewers.
- **Windows desktop app** — Electron wrapper around the live site with tray icon and automatic updates ([download](https://twintube.site/downloads/TwinTube-Setup.exe)).
- **Buy Me a Coffee** — Landing footer button + header logo link to [buymeacoffee.com/emireln](https://buymeacoffee.com/emireln).
- **Open source** — Header GitHub link to [github.com/emireln/twintube](https://github.com/emireln/twintube); see [CONTRIBUTING.md](CONTRIBUTING.md).
- **Docker ready** — Multi-stage image, Compose with PostgreSQL 16, production deploy helpers under `deploy/`.

---

## Technology Stack

| Layer | Technology |
| :--- | :--- |
| **Backend** | Go 1.21+, standard `net/http` |
| **Real-time** | WebSockets (`gorilla/websocket`) |
| **Database** | PostgreSQL (`lib/pq`) / SQLite (`modernc.org/sqlite`) |
| **Auth** | JWT + bcrypt |
| **Frontend** | HTML5, CSS3 (Material 3 tokens), ES6 modules |
| **Desktop** | Electron + electron-builder (NSIS) + electron-updater |
| **Voice** | Browser WebRTC + optional coturn |
| **Containers** | Docker / Docker Compose |

---

## Repository Structure

```
twintube/
├── main.go                 # Server entrypoint
├── Dockerfile
├── docker-compose.yml      # App + PostgreSQL
├── .env.example
├── CONTRIBUTING.md         # How to contribute
├── SECURITY.md             # Vulnerability reporting
├── AGENTS.md               # Architecture & coding conventions
├── desktop/                # Windows Electron client
├── downloads/              # Installer artifacts on VPS (mounted into app)
├── deploy/                 # Production compose, Caddy/nginx, coturn profile
├── scripts/                # Version bump, local start/stop, VPS helpers
├── internal/
│   ├── api/                # Rooms API, /api/rtc/config
│   ├── auth/               # Auth & profile
│   ├── db/                 # Dual DB + persistence
│   ├── room/               # Sync engine, roles, moments, join access
│   ├── security/           # Security headers & CSP
│   └── utils/              # Media URL helpers
└── static/
    ├── index.html          # Landing
    ├── room.html           # Watch room
    ├── desktop-logo.png / tray.ico
    ├── bmc-button.png / bmc-logo.svg
    ├── gifs/               # Reaction assets
    ├── css/style.css
    └── js/                 # app, player, ws, ui, voice, localmedia, i18n, …
```

---

## Quick Start

### Local development

Prerequisites: Go 1.21+.

```bash
git clone https://github.com/emireln/twintube.git
cd twintube
go run .
```

Open [http://localhost:8080](http://localhost:8080).

### Windows desktop app

```bash
cd desktop
npm ci
npm start      # opens the live TwinTube site in a desktop shell
npm run dist   # builds TwinTube-Setup-<version>.exe
```

Public download (after deploy): [https://twintube.site/downloads/TwinTube-Setup.exe](https://twintube.site/downloads/TwinTube-Setup.exe)

### Docker Compose (production-style)

```bash
git clone https://github.com/emireln/twintube.git
cd twintube
cp .env.example .env
# Set JWT_SECRET (and DB credentials if needed)
docker compose up -d --build
```

TwinTube listens on port `8080` with PostgreSQL 16.

### Optional voice (TURN)

Set `TURN_HOST` / `TURN_SECRET` in `.env`, open UDP/TCP **3478** and UDP **49152–49200**, then use the production compose **voice** profile (see `deploy/docker-compose.prod.yml` and `AGENTS.md`).

---

## Support the Creator

If you enjoy TwinTube:

[<img src="https://img.shields.io/badge/Buy%20Me%20A%20Coffee-Donate-FFDD00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee">](https://buymeacoffee.com/emireln)

---

## Contributing

Bug reports, ideas, and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md). Please report security issues privately per [SECURITY.md](SECURITY.md).

---

## WEBSITE

Currently, there is no website for the application, the VPS was for development tests; it is designed for self-hosting. 💡

## This application was developed while I was studying Go and WebSockets, with the help of AI (Cursor & OxAlpha).
