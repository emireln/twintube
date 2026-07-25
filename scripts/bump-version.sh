#!/usr/bin/env bash
# Bump patch version in VERSION file (1.0.0 -> 1.0.1)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$ROOT/VERSION"

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "1.0.0" > "$VERSION_FILE"
fi

CURRENT="$(tr -d '[:space:]' < "$VERSION_FILE")"
IFS='.' read -r MAJOR MINOR PATCH <<< "${CURRENT}.0.0"
PATCH=$((PATCH + 1))
NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"

echo "$NEW_VERSION" > "$VERSION_FILE"
echo "$NEW_VERSION"
