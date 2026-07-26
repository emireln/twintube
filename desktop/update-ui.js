const { BrowserWindow, ipcMain, nativeTheme } = require('electron');
const path = require('path');

const IPC_ACTION = 'update-ui:action';
const IPC_READY = 'update-ui:ready';
const IPC_STATE = 'update-ui:state';

class UpdateUI {
  constructor({ getParentWindow, getLang, translate }) {
    this.getParentWindow = getParentWindow;
    this.getLang = getLang;
    this.t = translate;
    this.win = null;
    this.pendingResolve = null;
    this.pendingReject = null;
    this.downloadingVersion = '';
    this.boundHandlers = false;
  }

  overlayPath(name) {
    return path.join(__dirname, name);
  }

  ensureIpc() {
    if (this.boundHandlers) return;
    this.boundHandlers = true;

    ipcMain.on(IPC_ACTION, (event, action) => {
      if (!this.win || event.sender !== this.win.webContents) return;
      if (!['download', 'later', 'restart', 'dismiss'].includes(action)) return;

      const resolve = this.pendingResolve;
      this.pendingResolve = null;
      this.pendingReject = null;

      if (resolve) resolve(action);

      if (action === 'later' || action === 'dismiss') {
        this.close();
      }
    });
  }

  async ensureWindow() {
    this.ensureIpc();
    if (this.win && !this.win.isDestroyed()) return this.win;

    const parent = this.getParentWindow?.();
    this.win = new BrowserWindow({
      width: 440,
      height: 320,
      resizable: false,
      minimizable: false,
      maximizable: false,
      fullscreenable: false,
      show: false,
      frame: false,
      transparent: true,
      parent: parent && !parent.isDestroyed() ? parent : undefined,
      modal: !!(parent && !parent.isDestroyed()),
      alwaysOnTop: true,
      skipTaskbar: true,
      backgroundColor: '#00000000',
      webPreferences: {
        preload: this.overlayPath('update-overlay-preload.js'),
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: true,
        webSecurity: true,
        allowRunningInsecureContent: false,
        devTools: false,
        spellcheck: false
      }
    });

    this.win.setMenuBarVisibility(false);
    this.win.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));

    this.win.webContents.on('will-navigate', (event) => {
      event.preventDefault();
    });

    await this.win.loadFile(this.overlayPath('update-overlay.html'));

    await new Promise((resolve) => {
      const timer = setTimeout(resolve, 5000);
      const onReady = (event) => {
        if (this.win && event.sender === this.win.webContents) {
          clearTimeout(timer);
          ipcMain.removeListener(IPC_READY, onReady);
          resolve();
        }
      };
      ipcMain.on(IPC_READY, onReady);
    });

    this.win.once('closed', () => {
      this.win = null;
      if (this.pendingReject) {
        this.pendingReject(new Error('update ui closed'));
        this.pendingResolve = null;
        this.pendingReject = null;
      }
    });

    return this.win;
  }

  pushState(state) {
    if (!this.win || this.win.isDestroyed()) return;
    this.win.webContents.send(IPC_STATE, {
      theme: nativeTheme.shouldUseDarkColors ? 'dark' : 'light',
      ...state
    });
  }

  async showState(state) {
    await this.ensureWindow();
    this.pushState(state);
    if (!this.win.isDestroyed()) {
      this.win.show();
      this.win.focus();
    }
  }

  close() {
    if (this.win && !this.win.isDestroyed()) {
      this.win.close();
    }
    this.win = null;
  }

  lang() {
    return this.getLang?.() || 'en';
  }

  waitForAction() {
    return new Promise((resolve, reject) => {
      this.pendingResolve = resolve;
      this.pendingReject = reject;
    });
  }

  async promptAvailable(version) {
    const lang = this.lang();
    const action = await (async () => {
      await this.showState({
        phase: 'available',
        version,
        title: this.t('update_available_title', {}, lang),
        body: this.t('update_available_body', { version }, lang),
        primaryLabel: this.t('update_download', {}, lang),
        secondaryLabel: this.t('update_later', {}, lang),
        primaryAction: 'download',
        secondaryAction: 'later',
        showProgress: false,
        showSecondary: true
      });
      return this.waitForAction();
    })();
    if (action !== 'download') this.close();
    return action;
  }

  showDownloading(version) {
    this.downloadingVersion = version || '';
    const lang = this.lang();
    return this.showState({
      phase: 'downloading',
      version: this.downloadingVersion,
      title: this.t('update_downloading_title', {}, lang),
      body: this.t('update_downloading_body', { version: this.downloadingVersion }, lang),
      progressLabel: this.t('update_progress_label', {}, lang),
      showProgress: true,
      showActions: false,
      progress: 0
    });
  }

  updateProgress(percent) {
    const lang = this.lang();
    const pct = Math.max(0, Math.min(100, Number(percent) || 0));
    this.pushState({
      phase: 'downloading',
      version: this.downloadingVersion,
      title: this.t('update_downloading_title', {}, lang),
      body: this.t('update_downloading_body', { version: this.downloadingVersion }, lang),
      progressLabel: this.t('update_progress_label', {}, lang),
      showProgress: true,
      showActions: false,
      progress: pct
    });
  }

  async promptReady(version) {
    const lang = this.lang();
    const action = await (async () => {
      await this.showState({
        phase: 'ready',
        version,
        title: this.t('update_ready_title', {}, lang),
        body: this.t('update_ready_body', { version }, lang),
        primaryLabel: this.t('update_restart', {}, lang),
        secondaryLabel: this.t('update_later', {}, lang),
        primaryAction: 'restart',
        secondaryAction: 'later',
        showProgress: false,
        showSecondary: true
      });
      return this.waitForAction();
    })();
    if (action !== 'restart') this.close();
    return action;
  }

  async showError(messageKey, vars = {}) {
    const lang = this.lang();
    await this.showState({
      phase: 'error',
      title: this.t('update_error_title', {}, lang),
      body: this.t(messageKey, vars, lang),
      primaryLabel: this.t('update_dismiss', {}, lang),
      secondaryLabel: this.t('update_later', {}, lang),
      primaryAction: 'dismiss',
      secondaryAction: 'dismiss',
      showProgress: false,
      showSecondary: false
    });
    await this.waitForAction().catch(() => 'dismiss');
    this.close();
  }
}

module.exports = { UpdateUI };
