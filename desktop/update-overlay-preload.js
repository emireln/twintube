const { contextBridge, ipcRenderer } = require('electron');

const ALLOWED_ACTIONS = new Set(['download', 'later', 'restart', 'dismiss']);

contextBridge.exposeInMainWorld('updateOverlay', {
  sendAction(action) {
    if (!ALLOWED_ACTIONS.has(action)) return;
    ipcRenderer.send('update-ui:action', action);
  },
  onState(callback) {
    if (typeof callback !== 'function') return;
    ipcRenderer.on('update-ui:state', (_event, state) => {
      callback(state);
    });
  },
  notifyReady() {
    ipcRenderer.send('update-ui:ready');
  }
});
