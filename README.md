<p align="center">
  <img src="static/logo.svg" alt="TwinTube Logo" width="140" height="140">
</p>

<h1 align="center">TwinTube</h1>

<p align="center">
  <b>Real-Time Synchronized Video Watching Platform</b><br>
  Inspired by Google Material Design 3 • Built with Go, WebSockets & PostgreSQL
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/Database-PostgreSQL%20%7C%20SQLite-4169E1?style=flat&logo=postgresql&logoColor=white" alt="Database">
  <img src="https://img.shields.io/badge/WebSockets-Gorilla-FF69B4?style=flat" alt="WebSockets">
  <img src="https://img.shields.io/badge/Styling-Material%20Design%203-757575?style=flat&logo=google" alt="Material Design 3">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue.svg" alt="License">
</p>

---

## 🌟 Features

- ⚡ **Server-Authoritative Sync Engine**: Ultra-low latency playback synchronization over WebSockets with automatic drift correction (**>1.5s tolerance** auto-seek).
- 📺 **YouTube iFrame API Integration**: Seamless playback control with auto-advance to the next video when current playback ends.
- 📜 **Collaborative Queue**: Paste YouTube links or Video IDs. Automatically fetches video titles, author names, and thumbnails via oEmbed.
- 💬 **Live Chat & System Alerts**: Real-time timestamped chat messages and automated system activity alerts (*"Alex paused the video"*).
- 🔐 **User Auth & Guest Access**: Register/Login using **JWT** tokens and **bcrypt** password hashing, or **Join as Guest** without registration.
- 🐘 **PostgreSQL & SQLite Dual Engine**: Native PostgreSQL connection for production VPS deployments, with zero-config SQLite fallback.
- 🎨 **Google Material 3 Aesthetics**: Google Blue (`#1a73e8`), elevation shadows, responsive 70%/30% grid desktop layout, and light/dark theme toggle.
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

## 📂 Repository Structure

```
twintube/
├── main.go              # Web server setup, routes, static server, WS handler & Auth API
├── auth.go              # User Registration, Login, JWT generation/validation, password hashing
├── room.go              # Room engine, client connection manager, server-authoritative sync
├── db.go                # PostgreSQL / SQLite dual driver setup & schema migrations
├── utils.go             # Helper utilities (ID generator, YouTube URL parser, oEmbed metadata)
├── Dockerfile           # Multi-stage production container build
├── docker-compose.yml   # Production Compose configuration for TwinTube + PostgreSQL 16
├── .env.example         # Production environment configuration template
└── static/
    ├── logo.svg         # Transparent Google Material 3 app logo
    ├── favicon.svg      # SVG browser favicon
    ├── index.html       # Single-page application layout
    ├── css/
    │   └── style.css    # Material Design 3 theme tokens & responsive styles
    └── js/
        ├── app.js       # Main application orchestrator
        ├── auth.js      # Client authentication manager
        ├── player.js    # YouTube iFrame API player controller & drift sync engine
        ├── ws.js        # Low-latency WebSocket client with auto-reconnect
        └── ui.js        # DOM rendering for chat, queue, audience roster, and toasts
```

---

## 🚀 Quick Start

### 1. Running Locally (Development Mode)

Prerequisites: Go 1.21+ installed on your system.

```bash
# Clone repository
git clone https://github.com/emireln/twintube.git
cd twintube

# Run application (defaults to local SQLite database twintube.db)
go run .
```

Open your browser at `http://localhost:8080` or `http://localhost:8080/room/abc123`.

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

## ⚙️ Environment Variables

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Port for the HTTP & WebSocket server |
| `DB_TYPE` | `sqlite` | Database engine (`postgres` or `sqlite`) |
| `DATABASE_URL` | - | PostgreSQL connection URL string |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `twintube` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `twintube` | PostgreSQL database name |
| `JWT_SECRET` | `twintube_default_secret` | Secret key used for signing JWT tokens |

---

## 📄 License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0). See the [LICENSE](LICENSE) file for details.
