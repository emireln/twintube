# TwinTube VPS Deployment

Deploy **twintube.site** with Docker + Caddy + GitHub Actions.

## Architecture

| Domain | Purpose |
|--------|---------|
| `twintube.site` | Full app (Go API + WebSocket + watch rooms) |
| `www.twintube.site` | App via Caddy |
| `home.twintube.site` | Redirects to `twintube.site` (legacy subdomain) |

Stack on VPS: **Caddy** (HTTPS) → **TwinTube** (Go) → **PostgreSQL 16**

---

## One-time VPS setup

### 1. DNS (at your registrar)

| Type | Name | Value |
|------|------|-------|
| A | `@` | `YOUR_VPS_IP` |
| A | `www` | `YOUR_VPS_IP` |

### 2. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

### 3. Clone repo

```bash
sudo mkdir -p /opt/twintube
sudo chown $USER:$USER /opt/twintube
git clone https://github.com/emireln/twintube.git /opt/twintube
cd /opt/twintube
cp deploy/.env.production.example .env
nano .env
```

Generate secrets:

```bash
openssl rand -hex 32   # JWT_SECRET
openssl rand -hex 24   # POSTGRES_PASSWORD
```

### 4. First deploy

```bash
mkdir -p downloads
chmod +x scripts/deploy-vps.sh
./scripts/deploy-vps.sh
```

Desktop installers are stored in `/opt/twintube/downloads` and mounted read-only into the app container at `/downloads`. GitHub Actions uploads `TwinTube-Setup.exe`, the versioned installer, `latest.yml`, and blockmap files on each deploy.

---

## GitHub Secrets (required)

**Settings → Secrets and variables → Actions → New repository secret**

| Secret | Example | Description |
|--------|---------|-------------|
| `VPS_HOST` | `123.45.67.89` | VPS IP or hostname |
| `VPS_USER` | `deploy` | SSH username |
| `VPS_SSH_KEY` | private key PEM | Full private key for deploy |
| `VPS_APP_DIR` | `/opt/twintube` | Repo path on VPS |
| `VPS_SSH_PORT` | `22` | Optional SSH port |

`GITHUB_TOKEN` is automatic (version bump commits).

**Repo setting:** **Settings → Actions → General → Workflow permissions → Read and write permissions** (required so the bump job can push `VERSION`).

### VPS-only secrets (in `.env`, never in GitHub)

| Variable | Description |
|----------|-------------|
| `POSTGRES_PASSWORD` | Database password |
| `JWT_SECRET` | JWT signing key |
| `ACME_EMAIL` | Let's Encrypt email |
| `DATABASE_URL` | Postgres connection URL |
| `DOWNLOADS_DIR` | Optional; container path for installers (default `/downloads`) |

---

## SSH deploy key

```bash
ssh-keygen -t ed25519 -C "github-twintube-deploy" -f ~/.ssh/twintube_deploy -N ""
```

Add public key to VPS `~/.ssh/authorized_keys`. Put private key in GitHub secret `VPS_SSH_KEY`.

The VPS must have **git** installed and the repo cloned so `git fetch origin main` works (public repo over HTTPS is fine).

---

## Auto deploy

Push to `main` runs:

1. Patch-bump `VERSION`
2. Build Windows NSIS installer on `windows-latest`
3. SCP installer + `latest.yml` to `$VPS_APP_DIR/downloads`
4. SSH deploy (`scripts/deploy-vps.sh`)
5. Verify `/api/version` and `/downloads/TwinTube-Setup.exe`

Manual redeploy on VPS:

```bash
cd /opt/twintube && git pull && ./scripts/deploy-vps.sh
```
