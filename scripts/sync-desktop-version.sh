#!/usr/bin/env bash
# Sync desktop/package.json version with root VERSION file.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
node -e "
const fs = require('fs');
const path = require('path');
const pkgPath = path.join(process.argv[1], 'desktop', 'package.json');
const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
pkg.version = process.argv[2];
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + '\n');
console.log('desktop version -> ' + process.argv[2]);
" "$ROOT" "$VERSION"
