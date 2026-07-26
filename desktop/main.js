const {
  app,
  BrowserWindow,
  Tray,
  Menu,
  shell,
  session,
  nativeImage
} = require('electron');
const path = require('path');
const fs = require('fs');
const { autoUpdater } = require('electron-updater');
const { detectLang, t } = require('./i18n');
const { UpdateUI } = require('./update-ui');

const APP_URL = process.env.TWINTUBE_DESKTOP_URL || 'https://twintube.site/?desktop=1';
const UPDATE_FEED = process.env.TWINTUBE_UPDATE_URL || 'https://twintube.site/downloads';

let mainWindow = null;
let tray = null;
let lang = detectLang();
let updatePromptOpen = false;
let updateUI = null;

function assertSecureOrigin(urlString, label) {
  if (!app.isPackaged) return;
  const url = new URL(urlString);
  if (url.protocol !== 'https:') {
    throw new Error(`${label} must use HTTPS in production`);
  }
}

function assetPath(name) {
  if (app.isPackaged) {
    const unpacked = path.join(process.resourcesPath, name);
    if (fs.existsSync(unpacked)) return unpacked;
    return path.join(__dirname, 'assets', name);
  }
  return path.join(__dirname, 'assets', name);
}

function hardenWebContents(contents) {
  contents.on('will-attach-webview', (event) => event.preventDefault());
  if (app.isPackaged) {
    contents.on('devtools-opened', () => contents.closeDevTools());
  }
}

function configureSession() {
  const ses = session.defaultSession;
  ses.setPermissionRequestHandler((_webContents, _permission, callback) => {
    callback(false);
  });
  ses.setPermissionCheckHandler(() => false);
  ses.setDevicePermissionHandler(() => false);
}

function createWindow() {
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
      webSecurity: true,
      allowRunningInsecureContent: false,
      devTools: !app.isPackaged,
      spellcheck: false
    }
  });

  hardenWebContents(mainWindow.webContents);

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

function getUpdateUI() {
  if (!updateUI) {
    updateUI = new UpdateUI({
      getParentWindow: () => mainWindow,
      getLang: () => lang,
      translate: t
    });
  }
  return updateUI;
}

function setupAutoUpdater() {
  if (!app.isPackaged) {
    console.log('[desktop] Skipping auto-updater in development.');
    return;
  }

  const ui = getUpdateUI();

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
    try {
      const action = await ui.promptAvailable(version);
      if (action === 'download') {
        await ui.showDownloading(version);
        await autoUpdater.downloadUpdate();
      }
    } catch (err) {
      console.error('[desktop] Download failed:', err);
      await ui.showError('update_error_download');
    } finally {
      updatePromptOpen = false;
    }
  });

  autoUpdater.on('download-progress', (progress) => {
    const pct = progress?.percent ?? 0;
    ui.updateProgress(pct);
  });

  autoUpdater.on('update-downloaded', async (info) => {
    if (updatePromptOpen) return;
    updatePromptOpen = true;
    const version = info?.version || '';
    try {
      const action = await ui.promptReady(version);
      if (action === 'restart') {
        app.isQuiting = true;
        autoUpdater.quitAndInstall(false, true);
      }
    } finally {
      updatePromptOpen = false;
    }
  });

  autoUpdater.on('error', async (err) => {
    console.error('[desktop] Updater error:', err);
    if (!updatePromptOpen) return;
    updatePromptOpen = true;
    try {
      await ui.showError('update_error_check');
    } finally {
      updatePromptOpen = false;
    }
  });

  const checkUpdates = () => {
    autoUpdater.checkForUpdates().catch(async (err) => {
      console.warn('[desktop] Update check failed:', err?.message || err);
    });
  };

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
    try {
      assertSecureOrigin(APP_URL, 'APP_URL');
      assertSecureOrigin(UPDATE_FEED, 'UPDATE_FEED');
    } catch (err) {
      console.error('[desktop] Security configuration error:', err.message);
      app.quit();
      return;
    }

    configureSession();
    lang = detectLang();

    app.on('web-contents-created', (_event, contents) => {
      hardenWebContents(contents);
    });

    createWindow();
    createTray();
    setupAutoUpdater();
  });

  app.on('before-quit', () => {
    app.isQuiting = true;
    if (updateUI) updateUI.close();
  });

  app.on('window-all-closed', (e) => {
    e.preventDefault();
  });
}

module.exports = { APP_URL, UPDATE_FEED };
