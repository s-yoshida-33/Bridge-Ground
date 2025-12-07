// src/ui/preload.js
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('bridgeApi', {
    getConfig: () => ipcRenderer.invoke('get-config'),
    saveConfig: (config) => ipcRenderer.invoke('save-config', config),
    startManualSync: () => ipcRenderer.invoke('start-manual-sync'),
    // Expose channels for receiving sync status updates (e.g., 'sync-status-update')
});