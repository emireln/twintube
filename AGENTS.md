# AGENTS.md - TwinTube Developer & AI Agent Guidelines

Welcome to the **TwinTube** project repository! This document provides technical guidelines, architecture overviews, coding conventions, and instructions for AI coding assistants working in this workspace.

---

## Project Overview

**TwinTube** is a lightweight, real-time synchronized video watching web application inspired by Google Material Design 3 and SyncTube. Users create public/private rooms, watch YouTube / Vimeo / Twitch / direct / local media together with server-authoritative sync, chat with avatars, collaborate on a queue, save jumpable moments, send GIF reactions, and optionally use push-to-talk WebRTC voice.

Rooms are **fully open by default**. Hosts can tighten permissions (playback, add/mod queue, moments, jump) via `SET_ROOM_PERMISSIONS`.

**Guest rooms** (no signed-in owner) are in-memory only. When the last viewer leaves, the room stays joinable for **15 minutes** (`GuestRoomGracePeriod` in `internal/room/room.go`); rejoining cancels the timer. Signed-in rooms persist to the DB for 7 days.

---

## Technology Stack & Architecture

### Backend (Go)
- **Language**: Go 1.21+
- **HTTP & WebSockets**: `net/http` + `github.com/gorilla/websocket`
- **Database**: PostgreSQL 16 (`github.com/lib/pq`) for production, SQLite (`modernc.org/sqlite`) fallback
- **Auth & Security**: bcrypt, JWT (`jwt/v5`), security headers + CSP (`internal/security`)
- **State**: In-memory thread-safe `RoomManager` / `Room` with optional DB persistence
- **Voice**: Optional STUN/TURN via `/api/rtc/config` + coturn compose profile
- **Desktop**: Electron Windows client wraps `https://twintube.site/?desktop=1` with tray + auto-update

### Frontend
- **Pages**: Landing (`static/index.html`) & Watch Room (`static/room.html`)
- **Styling**: Vanilla CSS3 Material 3 tokens; desktop + mobile-first room chrome (left drawer, bottom tabs ≤900px)
- **Modules**: `landing.js`, `app.js`, `auth.js`, `player.js`, `ws.js`, `ui.js`, `i18n.js`, `localmedia.js`, `voice.js`
- **i18n**: English + Portuguese (`static/js/i18n.js`)
- **Players**: YouTube iFrame API, HTML5 video, generic iframe embeds
- **Desktop flag**: `?desktop=1` / `window.twintubeDesktop` hides landing hero subtitle + Windows download CTA

---

## Repository Layout

```
twintube/
├── main.go                 # Canonical server entrypoint (HTTP + WS)
├── go.mod / go.sum
├── Dockerfile
├── docker-compose.yml      # App + PostgreSQL 16
├── .env.example
├── desktop/                # Electron Windows client (NSIS setup.exe)
├── downloads/              # Runtime installer artifacts (VPS volume; not in image)
├── deploy/                 # Production Caddy/nginx + compose with optional coturn
├── internal/
│   ├── api/                # REST: rooms, RTC config
│   ├── auth/               # Register / login / profile / JWT
│   ├── db/                 # Dual driver + migrations + chat/playlist persistence
│   ├── room/               # Room engine, roles, moments, join access
│   ├── security/           # Headers, CSP, Permissions-Policy
│   ├── utils/              # IDs, URL/media extract, oEmbed
│   └── version/
└── static/
    ├── index.html / room.html
    ├── desktop-logo.png    # Desktop / installer branding
    ├── tray.ico            # Desktop tray icon
    ├── bmc-button.png      # Landing page Buy Me a Coffee CTA
    ├── bmc-logo.svg        # Room-header support icon
    ├── gifs/               # Reaction WebPs
    ├── css/style.css
    └── js/                 # Frontend modules (incl. voice.js, localmedia.js)
```

> Prefer root `main.go` as the source of truth. Ignore or delete stale `cmd/server` if it reappears.

### Desktop Windows client
- Loads live site with `?desktop=1` (no bundled Go server).
- Tray menu EN/PT; auto-updater reads `https://twintube.site/downloads/latest.yml`.
- Build: `cd desktop && npm ci && npm run dist` → `TwinTube-Setup-<version>.exe`.
- CI publishes stable `/downloads/TwinTube-Setup.exe` + versioned installer + `latest.yml` to the VPS `downloads/` volume (mounted read-only at `/downloads` in the app container).
- Server env: `DOWNLOADS_DIR` (default `./downloads`).

---

## WebSocket Protocol (Room Features)

| Action | Direction | Purpose |
|--------|-----------|---------|
| `JOIN_ROOM` | C→S | `{roomId, nickname, token?, joinToken?}` |
| `JOIN_PRESENCE` | C→S | `{token}` — logged-in landing/background alerts (no room join) |
| `INIT_STATE` | S→C | Snapshot: video, playlist, users, roles, permissions, moments |
| `CHAT_MESSAGE` | C↔S | `{content, replyToId?, videoTime?}` → message with `id`, `mentions`, reply preview |
| `CHAT_REACTION` | C→S | `{messageId, emoji, videoTime?}` toggle emoji on a chat line |
| `CHAT_REACTION_UPDATE` | S→C | `{messageId, reactions, emoji, added, clientId}` |
| `HYPE_BURST` | S→C | `{emoji, videoTime, count, label}` when many react at same timestamp |
| `MENTION_NOTIFY` | S→C | `{roomId, messageId, from, content}` targeted @mention alert |
| `COWATCHER_ROOM` | S→C | `{roomId, roomName, hostNickname, url}` co-watcher started a room |
| `CHAT_HISTORY` | S→C | Last N messages (oldest→newest) with avatars when known |
| `ADD_QUEUE` | C→S | `{url, title?}` → parse → append |
| `QUEUE_UPDATE` | S→C | Full playlist |
| `PLAY_QUEUE_ITEM` / `REMOVE_QUEUE_ITEM` / `REORDER_QUEUE` | C→S | Queue control (gated by permissions) |
| `STATE_CHANGE` | C→S | Local play/pause/seek → server authority |
| `STATE_UPDATE` | S→C | Authoritative video state |
| `SYNC_REQUEST` | C→S | Force personal resync |
| `VIDEO_REACTION` | C↔S | Allowlisted: `happy`, `energetic`, `stressed`, `tired`, `heart`, `lmao`, `popcorn`, `monkey-no-look` (`.webp`) |
| `CLEAR_CURRENT_VIDEO` | C→S | Host-only; empty the current player (no video until someone adds one) |
| `SET_ROOM_PERMISSIONS` | C→S | Host-only; open-by-default viewer flags |
| `USER_LIST` / `TRANSFER_HOST` / `GRANT_COHOST` / `REVOKE_COHOST` | ↔ | Audience + roles |
| `SUBMIT_MOMENT` / `APPROVE_MOMENT` / `REJECT_MOMENT` / `JUMP_TO_MOMENT` | ↔ | Timestamp moments |
| `VOICE_JOIN` / `VOICE_LEAVE` / `VOICE_STATUS` | C→S | Push-to-talk presence |
| `RTC_OFFER` / `RTC_ANSWER` / `RTC_ICE` | C→S→C | Targeted WebRTC signaling (`SendTo`) |
| `LOCAL_FILE_STATUS` | C→S | Local Night readiness |
| `ROOM_META` | S→C | Permissions / capability refresh |
| `ERROR` | S→C | Client-visible errors |

### Room permissions (defaults all `true`)
- `anyonePlayback` — pause / play / `STATE_CHANGE`
- `anyoneAddQueue` — add videos / local files
- `anyoneModQueue` — remove / reorder
- `anyoneMoment` — submit moments
- `anyoneJump` — jump everyone to a moment  
Host/co-host always bypass. Approve/reject moments remain controller-only.

---

## Coding Conventions

### Backend (Go)
1. Business logic in `internal/`; root `main.go` is the server entrypoint.
2. Guard room state with `sync.RWMutex`. Prefer helpers (`AppendPlaylistItem`, `SnapshotState`, `TransferHost`, `SetPermissions`, …).
3. Never send on `Room.Broadcast` from inside `Room.Run` Register/Unregister — use `deliver()`.
4. Use `Database.Rebind` for `$n` / `?` dual SQL.
5. Hash passwords with bcrypt (cost 12).
6. In range loops, copy by value — never `&item` from `for _, item := range`.

### Sync / Player
1. Server-authoritative position:  
   `CurrentPosition = CurrentTime + (Now - ServerTimestamp)/1000` when `PLAYING`.
2. Drift **> 1.5s** → force seek.
3. `videoId` change → `loadVideoById` (seek-only is not enough).
4. Buffer state as `pendingServerState` until the player is ready.
5. Set `isRemoteUpdate` (~1.2–2s) to avoid echo fights.
6. On `ENDED`, auto `PLAY_QUEUE_ITEM` for the next queue entry when allowed.

### Voice
- Mesh PTT, muted by default; Space / hold button to talk (max ~6 peers).
- Deterministic offerer (`clientId` lexicographic) to avoid glare.
- Queue ICE until remote description exists; resume remote `<audio>` on gesture.
- Production NAT traversal needs coturn (`deploy` profile `voice`) + `TURN_*` env.

### UI notes
- Reactions: compact dock on the player (not the toolbar).
- Moments: bookmarks button → panel on the video meta card (not inline in chat).
- Mobile room (≤900px): header → left drawer; Chat/Queue/Viewers → bottom nav.
- Avatars: render `avatarUrl` in chat + viewers; fall back to initials.
- BMC: `bmc-button.png` on landing footer only; `bmc-logo.svg` in **room** header only.
- Landing Windows download CTA links to `/downloads/TwinTube-Setup.exe` (hidden in desktop app).

---

## Commands & Deployment

### Local
```bash
go run .
# or
go build -o twintube.exe .
./twintube.exe
```
Open `http://localhost:8080`.

### Windows desktop client
```bash
cd desktop
npm ci
npm start          # loads live site with ?desktop=1
npm run dist       # NSIS TwinTube-Setup-<version>.exe
```

### Docker (app + Postgres)
```bash
cp .env.example .env
docker compose up -d --build
```

### Production voice (optional)
```bash
# set TURN_HOST / TURN_SECRET in .env, then:
docker compose -f deploy/docker-compose.prod.yml --profile voice up -d
```
Open UDP/TCP **3478** and UDP **49152–49200** on the host firewall.
