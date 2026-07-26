# TwinTube Desktop (Electron)

Windows client that wraps https://twintube.site with tray + auto-update.

## Updates

The desktop app uses a Material-style in-app overlay for update prompts (available, download progress, ready to restart, and errors) with English and Portuguese strings in `i18n.js`. Updates are fetched only from HTTPS (`https://twintube.site/downloads`).

## Develop

```bash
cd desktop
npm ci
npm start
```

Optional local URL:

```bash
set TWINTUBE_DESKTOP_URL=http://localhost:8080/?desktop=1
npm start
```

## Build installer

```bash
cd desktop
# Sync version with root VERSION first (CI does this)
npm ci
npm run dist
```

Output: `desktop/dist/TwinTube-Setup-<version>.exe` and `latest.yml`.
