# TwinTube Desktop (Electron)

Windows client that wraps https://twintube.site with tray + auto-update.

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
