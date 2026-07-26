const {
  app,
  BrowserWindow,
  Tray,
  Menu,
  shell,
  dialog,
  nativeImage
} = require('electron');
const path = require('path');
const fs = require('fs');
const { autoUpdater } = require('electron-updater');
const { detectLang, t } = require('./i18n');

const APP_URL = process.env.TWINTUBE_DESKTOP_URL || 'https://twintube.site/?desktop=1';
const UPDATE_FEED = process.env.TWINTUBE_UPDATE_URL || 'https://twintube.site/downloads';

let mainWindow = null;
let tray = null;
let lang = detectLang();
let updatePromptOpen = false;

function assetPath(name) {
  if (app.isPackaged) {
    const unpacked = path.join(process.resourcesPath, name);
    if (fs.existsSync(unpacked)) return unpacked;
    return path.join(__dirname, 'assets', name);
  }
  return path.join(__dirname, 'assets', name);
}

function createWindow() {
  // Prefer multi-size .ico so Windows taskbar / title-bar stay sharp; PNG is a fallback.
  const ico = nativeImage.createFromPath(assetPath('icon.ico'));
  const png = nativeImage.createFromPath(assetPath('icon.png'));
  const winIcon = !ico.isEmpty() ? ico : png;
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 840,
    minWidth: 960,
    minHeight: 640,
    title: 'TwinTube',
    icon: winIcon.isEmpty() ? undefined : winIcon,
    show: false,
    backgroundColor: '#121316',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      spellcheck: false
    }
  });

  mainWindow.once('ready-to-show', () => {
    if (mainWindow) mainWindow.show();
  });

  mainWindow.on('close', (event) => {
    if (!app.isQuiting) {
      event.preventDefault();
      mainWindow.hide();
    }
  });

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: 'deny' };
  });

  mainWindow.webContents.on('will-navigate', (event, url) => {
    try {
      const target = new URL(url);
      const allowed = new URL(APP_URL);
      if (target.origin !== allowed.origin) {
        event.preventDefault();
        shell.openExternal(url);
      }
    } catch (_) {
      event.preventDefault();
    }
  });

  mainWindow.loadURL(APP_URL);
}

function buildTrayMenu() {
  return Menu.buildFromTemplate([
    {
      label: t('show', {}, lang),
      click: () => {
        if (!mainWindow) return;
        mainWindow.show();
        mainWindow.focus();
      }
    },
    {
      label: t('hide', {}, lang),
      click: () => {
        if (mainWindow) mainWindow.hide();
      }
    },
    { type: 'separator' },
    {
      label: t('quit', {}, lang),
      click: () => {
        app.isQuiting = true;
        app.quit();
      }
    }
  ]);
}

function createTray() {
  const trayIcon = nativeImage.createFromPath(assetPath('tray.ico'));
  tray = new Tray(trayIcon.isEmpty() ? nativeImage.createEmpty() : trayIcon);
  tray.setToolTip('TwinTube');
  tray.setContextMenu(buildTrayMenu());
  tray.on('double-click', () => {
    if (!mainWindow) return;
    mainWindow.show();
    mainWindow.focus();
  });
}

function setupAutoUpdater() {
  if (!app.isPackaged) {
    console.log('[desktop] Skipping auto-updater in development.');
    return;
  }

  autoUpdater.autoDownload = false;
  autoUpdater.autoInstallOnAppQuit = true;
  autoUpdater.setFeedURL({
    provider: 'generic',
    url: UPDATE_FEED
  });

  autoUpdater.on('update-available', async (info) => {
    if (updatePromptOpen) return;
    updatePromptOpen = true;
    const version = info?.version || '';
    const result = await dialog.showMessageBox(mainWindow || undefined, {
      type: 'info',
      buttons: [t('update_download', {}, lang), t('update_later', {}, lang)],
      defaultId: 0,
      cancelId: 1,
      title: t('update_available_title', {}, lang),
      message: t('update_available_body', { version }, lang)
    });
    updatePromptOpen = false;
    if (result.response === 0) {
      try {
        await autoUpdater.downloadUpdate();
      } catch (err) {
        console.error('[desktop] Download failed:', err);
      }
    }
  });

  autoUpdater.on('download-progress', () => {
    // Progress is handled silently; dialog appears when ready.
  });

  autoUpdater.on('update-downloaded', async (info) => {
    if (updatePromptOpen) return;
    updatePromptOpen = true;
    const version = info?.version || '';
    const result = await dialog.showMessageBox(mainWindow || undefined, {
      type: 'info',
      buttons: [t('update_restart', {}, lang), t('update_later', {}, lang)],
      defaultId: 0,
      cancelId: 1,
      title: t('update_ready_title', {}, lang),
      message: t('update_ready_body', { version }, lang)
    });
    updatePromptOpen = false;
    if (result.response === 0) {
      app.isQuiting = true;
      autoUpdater.quitAndInstall(false, true);
    }
  });

  autoUpdater.on('error', (err) => {
    console.error('[desktop] Updater error:', err);
  });

  const checkUpdates = () => {
    autoUpdater.checkForUpdates().catch((err) => {
      console.warn('[desktop] Update check failed:', err?.message || err);
    });
  };

  // Check shortly after launch, then periodically while the app stays open.
  setTimeout(checkUpdates, 4000);
  setInterval(checkUpdates, 6 * 60 * 60 * 1000);
}

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (!mainWindow) return;
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.show();
    mainWindow.focus();
  });

  app.whenReady().then(() => {
    lang = detectLang();
    createWindow();
    createTray();
    setupAutoUpdater();
  });

  app.on('before-quit', () => {
    app.isQuiting = true;
  });

  app.on('window-all-closed', (e) => {
    // Keep tray app alive on Windows/Linux until explicit quit.
    e.preventDefault();
  });
}
