#!/usr/bin/env bash
# Run on the VPS after git pull — rebuild and restart TwinTube stack
set -euo pipefail

APP_DIR="${VPS_APP_DIR:-/opt/twintube}"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/docker-compose.prod.yml}"

cd "$APP_DIR"

if [[ ! -f .env ]]; then
  echo "ERROR: $APP_DIR/.env missing. Copy deploy/.env.production.example to .env and configure secrets."
  exit 1
fi

export VERSION="$(tr -d '[:space:]' < VERSION)"
export GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
export BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "==> Deploying TwinTube v${VERSION} (${GIT_COMMIT})"

mkdir -p "$APP_DIR/downloads"

docker compose --env-file .env -f "$COMPOSE_FILE" pull --ignore-buildable 2>/dev/null || true
docker compose --env-file .env -f "$COMPOSE_FILE" build --no-cache
docker compose --env-file .env -f "$COMPOSE_FILE" up -d --remove-orphans
docker compose --env-file .env -f "$COMPOSE_FILE" ps

# Prune old images (keep last deploy lean)
docker image prune -f >/dev/null 2>&1 || true

echo "==> Deploy complete: https://twintube.site (v${VERSION})"
