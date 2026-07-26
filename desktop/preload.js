const { contextBridge, ipcRenderer } = require('electron');

ipcRenderer.on('twintube-notification-clicked', () => {
  window.dispatchEvent(new CustomEvent('twintube:notification-click'));
});

contextBridge.exposeInMainWorld('twintubeDesktop', {
  isDesktop: true,
  platform: process.platform,
  showNotification: ({ title, body, tag }) => {
    ipcRenderer.send('twintube-show-notification', { title, body, tag });
  }
});
