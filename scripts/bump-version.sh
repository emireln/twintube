#!/usr/bin/env bash
# Bump patch version in VERSION file (1.0.0 -> 1.0.1)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$ROOT/VERSION"

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "1.0.0" > "$VERSION_FILE"
fi

CURRENT="$(tr -d '[:space:]' < "$VERSION_FILE")"
MAJOR="$(echo "$CURRENT" | cut -d. -f1)"
MINOR="$(echo "$CURRENT" | cut -d. -f2)"
PATCH="$(echo "$CURRENT" | cut -d. -f3)"

MAJOR="${MAJOR:-1}"
MINOR="${MINOR:-0}"
PATCH="${PATCH:-0}"

PATCH=$((PATCH + 1))
NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"

echo "$NEW_VERSION" > "$VERSION_FILE"

# Keep the Electron package version aligned so local + CI builds match VERSION.
if [[ -f "$ROOT/scripts/sync-desktop-version.sh" ]]; then
  chmod +x "$ROOT/scripts/sync-desktop-version.sh"
  "$ROOT/scripts/sync-desktop-version.sh" >/dev/null
fi

echo "$NEW_VERSION"
