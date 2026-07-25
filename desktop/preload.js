const { contextBridge } = require('electron');

contextBridge.exposeInMainWorld('twintubeDesktop', {
  isDesktop: true,
  platform: process.platform
});
