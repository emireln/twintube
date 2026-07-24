<p align="center">
  <img src="static/logo.svg" alt="TwinTube Logo" width="140" height="140">
</p>

<h1 align="center">TwinTube</h1>

<p align="center">
  <b>Real-Time Synchronized Video Watching Platform</b><br>
  Inspired by SyncTube & Google Material Design 3 • Built with Go, WebSockets & PostgreSQL
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/Database-PostgreSQL%20%7C%20SQLite-4169E1?style=flat&logo=postgresql&logoColor=white" alt="Database">
  <img src="https://img.shields.io/badge/WebSockets-Gorilla-FF69B4?style=flat" alt="WebSockets">
  <img src="https://img.shields.io/badge/Styling-Material%20Design%203-757575?style=flat&logo=google" alt="Material Design 3">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue.svg" alt="License">
  <a href="https://buymeacoffee.com/emireln" target="_blank"><img src="https://img.shields.io/badge/Buy%20Me%20A%20Coffee-Donate-FFDD00?style=flat&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee"></a>
</p>

---

## 🌟 Features

- 🏠 **SyncTube-Style Landing Page**: Minimalist landing page with instant **Create Room** button, Log In / Sign Up actions, and creator support link.
- ⚡ **Server-Authoritative Sync Engine**: Ultra-low latency playback synchronization over WebSockets with automatic drift correction (**>1.5s tolerance** auto-seek).
- 📺 **YouTube iFrame API Integration**: Seamless playback control with auto-advance to the next video when current playback ends.
- 📜 **Collaborative Queue**: Paste YouTube links or Video IDs. Automatically fetches video titles, author names, and thumbnails via oEmbed.
- 💬 **Live Chat & System Alerts**: Real-time timestamped chat messages and automated system activity alerts (*"Alex paused the video"*).
- 🔐 **User Auth & Guest Access**: Register/Login using **JWT** tokens and **bcrypt** password hashing, or **Join as Guest** without registration.
- 🐘 **PostgreSQL & SQLite Dual Engine**: Native PostgreSQL connection for production VPS deployments, with zero-config SQLite fallback.
- 🎨 **Google Material 3 Aesthetics**: Google Blue (`#1a73e8`), elevation shadows, responsive 70%/30% grid desktop layout, and light/dark theme toggle.
- ☕ **Buy Me a Coffee Support**: Direct integrated link to support the creator at [buymeacoffee.com/emireln](https://buymeacoffee.com/emireln).
- 🐳 **Docker VPS Ready**: Includes multi-stage `Dockerfile` and `docker-compose.yml` orchestrating TwinTube + PostgreSQL 16.

---

## 🛠️ Technology Stack

| Layer | Technology |
| :--- | :--- |
| **Backend** | Go (Golang) 1.21+, Standard `net/http` |
| **Real-Time Engine** | WebSockets (`github.com/gorilla/websocket`) |
| **Database** | PostgreSQL (`github.com/lib/pq`) / SQLite (`modernc.org/sqlite`) |
| **Authentication** | JWT (`github.com/golang-jwt/jwt/v5`) & Bcrypt (`golang.org/x/crypto`) |
| **Frontend** | Pure HTML5, Vanilla CSS3 (CSS Variables), Vanilla ES6 Modules |
| **Containerization**| Docker & Docker Compose |

---

## 📂 Repository Structure (Standard Go Package Layout)

```
twintube/
├── main.go              # Root entry point delegating server execution
├── go.mod               # Go module definition
├── Dockerfile           # Multi-stage production container build
├── docker-compose.yml   # Production Compose configuration for TwinTube + PostgreSQL 16
├── .env.example         # Production environment configuration template
├── cmd/
│   └── server/
│       └── main.go      # Application server entrypoint and HTTP routes
├── internal/
│   ├── auth/
│   │   └── auth.go      # User Registration, Login, JWT generation/validation
│   ├── db/
│   │   └── db.go        # PostgreSQL / SQLite dual driver setup & schema migrations
│   ├── room/
│   │   └── room.go      # Room engine & server-authoritative sync logic
│   └── utils/
│       └── utils.go     # Helper utilities (oEmbed metadata, ID generation)
└── static/
    ├── logo.svg         # Transparent Google Material 3 app logo
    ├── favicon.svg      # SVG browser favicon
    ├── index.html       # SyncTube-style Landing Page layout
    ├── room.html        # Watch Room layout
    ├── css/
    │   └── style.css    # Material Design 3 theme tokens & landing styles
    └── js/
        ├── landing.js   # Landing page controller
        ├── app.js       # Main watch room orchestrator
        ├── auth.js      # Client authentication manager
        ├── player.js    # YouTube iFrame API player controller & drift sync engine
        ├── ws.js        # Low-latency WebSocket client with auto-reconnect
        └── ui.js        # DOM rendering for chat, playlist queue, audience list, and toasts
```

---

## 🚀 Quick Start

### 1. Running Locally (Development Mode)

Prerequisites: Go 1.21+ installed on your system.

```bash
# Clone repository
git clone https://github.com/emireln/twintube.git
cd twintube

# Run application
go run .
```

Open your browser at `http://localhost:8080`.

---

### 2. VPS Deployment via Docker Compose (Production Mode)

1. Clone repository to your VPS:
   ```bash
   git clone https://github.com/emireln/twintube.git
   cd twintube
   ```

2. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

3. Configure your secret key in `.env`:
   ```env
   PORT=8080
   DB_TYPE=postgres
   DATABASE_URL=postgres://twintube_user:twintube_secure_password@postgres:5432/twintube_db?sslmode=disable
   JWT_SECRET=your_super_secret_vps_jwt_signing_key_here
   ```

4. Start Docker Compose:
   ```bash
   docker compose up -d --build
   ```

TwinTube will be running live on port `8080` backed by a dedicated PostgreSQL 16 container!

---

## ☕ Support the Creator

If you enjoy using TwinTube, consider supporting the project:

[<img src="https://img.shields.io/badge/Buy%20Me%20A%20Coffee-Donate-FFDD00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me A Coffee">](https://buymeacoffee.com/emireln)

---

## 📄 License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0). See the [LICENSE](LICENSE) file for details.
