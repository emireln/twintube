# AGENTS.md - TwinTube Developer & AI Agent Guidelines

Welcome to the **TwinTube** project repository! This document provides technical guidelines, architecture overviews, coding conventions, and instructions for AI coding assistants working in this workspace.

---

## 🚀 Project Overview

**TwinTube** is a lightweight, real-time synchronized video watching web application inspired by Google Material Design 3. It allows users to create public/private rooms, embed YouTube videos, watch in synchronized playback with low-latency WebSockets, chat with timestamped messages and system alerts, and queue videos collaboratively.

---

## 🛠️ Technology Stack & Architecture

### Backend (Go)
- **Language**: Go 1.21+
- **HTTP & WebSockets**: `net/http` standard library + `github.com/gorilla/websocket`
- **Database Engine**: PostgreSQL 16 (`github.com/lib/pq`) for production VPS hosting, with zero-config SQLite (`modernc.org/sqlite`) fallback.
- **Authentication & Security**: `golang.org/x/crypto/bcrypt` for password hashing, `github.com/golang-jwt/jwt/v5` for stateless JWT tokens, security headers middleware (`nosniff`, `SAMEORIGIN`).
- **State Management**: In-memory thread-safe room manager (`RoomManager`, `Room`) backed by database persistence.

### Frontend
- **Structure**: HTML5 (`static/index.html`)
- **Styling**: Vanilla CSS3 (`static/css/style.css`) using CSS variables for Google Material 3 themes (Light/Dark mode)
- **Logic**: Vanilla ES6+ JavaScript Modules (`app.js`, `auth.js`, `player.js`, `ws.js`, `ui.js`)
- **Video Player**: YouTube iFrame Player API (`YT.Player`)

---

## 📂 Repository Layout

```
twintube/
├── go.mod               # Go module definition (lib/pq, jwt/v5, crypto)
├── main.go              # Web server setup, routes, static server, WS upgrade & Auth API
├── auth.go              # User Registration, Login, JWT generation/validation, password hashing
├── room.go              # Room engine, client management, server-authoritative sync
├── db.go                # PostgreSQL / SQLite dual driver setup & schema migrations
├── utils.go             # Helper utilities (ID generator, YouTube URL parser, oEmbed metadata)
├── Dockerfile           # Multi-stage production container build for VPS
├── docker-compose.yml   # Production Compose configuration for TwinTube + PostgreSQL 16
├── .env.example         # Production environment configuration template
└── static/
    ├── index.html       # Single-page application layout with Auth & Nickname modals
    ├── css/
    │   └── style.css    # Material Design 3 theme tokens & responsive styles
    └── js/
        ├── app.js       # Main application orchestrator
        ├── auth.js      # Client authentication manager (Token storage, login/register API)
        ├── player.js    # YouTube iFrame API player controller & drift sync engine (>1.5s)
        ├── ws.js        # Low-latency WebSocket client with auto-reconnect
        └── ui.js        # DOM rendering for chat, playlist queue, audience list, and toasts
```

---

## 📐 Coding Conventions & Guidelines

### Backend (Go)
1. **Thread Safety**: All access to room state (`Room.State`, `Room.Clients`, `Room.Playlist`) must be guarded using `sync.RWMutex` (`RLock()` for reads, `Lock()` for state mutations).
2. **Database Integrity & Rebinding**: Use `Database.Rebind(query)` for all SQL parameters to automatically support `$1, $2` for PostgreSQL and `?` for SQLite.
3. **Password Security**: Always hash passwords using `bcrypt` (cost 12). Never store plaintext passwords.
4. **WebSocket Authentication**: Pass JWT `token` in `JOIN_ROOM` payload. If invalid or missing, gracefully fall back to `IsGuest = true` with guest nickname.

### Sync Logic & Player Engine
1. **Server-Authoritative Sync**: The backend holds the true video state (`videoId`, `status`, `currentTime`, `serverTimestamp`). The calculated current playback position is:
   $$\text{CurrentPosition} = \text{CurrentTime} + \frac{\text{Now} - \text{ServerTimestamp}}{1000} \quad (\text{if status == "PLAYING"})$$
2. **Drift Correction**: The client compares current player time with calculated server position. If drift is **> 1.5 seconds**, force a seek (`player.seekTo`).
3. **Auto-Advance**: When a video emits `ENDED` state, automatically pop and play the next video in the queue.

---

## 💻 Commands & VPS Deployment

### Running Locally
```bash
go run .
```

### VPS Deployment via Docker Compose
```bash
cp .env.example .env
docker compose up -d --build
```
