// src/main.js
const { app, BrowserWindow, Menu, Tray, ipcMain } = require('electron');
const path = require('path');
const ConfigManager = require('./config/ConfigManager');
const SyncManager = require('./sync/SyncManager');
const HttpServer = require('./server/HttpServer');
// Assuming FileUtil is implemented and includes getAppDataPath
// const { getAppDataPath } = require('./utils/FileUtil'); 

let mainWindow;
let appTray;
let syncManager;
let httpServer;

// --- Application Core Initialization ---

/**
 * @description Initializes the core backend services (Config, HTTP Server, Sync Manager).
 * Also handles initial and scheduled synchronization based on configuration.
 */
async function initializeCore() {
    try {
        // 1. Load Configuration
        const config = await ConfigManager.loadConfig();
        
        // 2. Initialize and Start HTTP Server
        httpServer = new HttpServer();
        httpServer.start();

        // 3. Initialize Sync Manager
        syncManager = new SyncManager();
        
        // 4. Handle initial synchronization (Sync on Startup)
        if (config.syncSettings.syncOnStartup) {
            console.log("Starting sync on startup...");
            await syncManager.startSync();
        }

        // 5. Handle scheduled synchronization (Auto Sync)
        if (config.syncSettings.autoSyncEnabled) {
            const intervalMinutes = config.syncSettings.syncIntervalMinutes;
            const intervalMs = intervalMinutes * 60 * 1000;
            
            // Start periodic synchronization
            setInterval(() => {
                console.log(`[Auto Sync] Triggered sync at interval of ${intervalMinutes} minutes.`);
                syncManager.startSync();
            }, intervalMs);
        }

    } catch (error) {
        console.error("Fatal error during application core initialization:", error.message);
        // On fatal error, notify UI (if exists) and prepare to quit if necessary.
    }
}


// --- Electron UI and Tray Functions ---

function createMainWindow() {
    // Create the browser window.
    mainWindow = new BrowserWindow({
        width: 800,
        height: 600,
        icon: path.join(__dirname, 'ui/icon.png'), // Placeholder icon
        show: false, // Start hidden to respect the system tray paradigm
        webPreferences: {
            preload: path.join(__dirname, 'ui/preload.js'), // Secure way to expose APIs to renderer
            nodeIntegration: false,
            contextIsolation: true // Security best practice
        }
    });

    // Load the index.html of the app.
    mainWindow.loadFile(path.join(__dirname, 'ui/index.html'));

    // Handle closing the window (Minimize to tray)
    mainWindow.on('close', (event) => {
        if (!app.isQuiting) {
            event.preventDefault();
            mainWindow.hide(); // Hide instead of closing
            event.returnValue = false;
        }
    });

    // Show the window only when content is ready (avoids flash)
    mainWindow.on('ready-to-show', () => {
        // Can be changed to mainWindow.show() if user prefers startup visibility
        // mainWindow.show(); 
    });
}

function createTray() {
    // Create the system tray icon
    const iconPath = path.join(__dirname, 'ui/tray_icon.png'); // Placeholder icon
    appTray = new Tray(iconPath);
    appTray.setToolTip('BridgeGround Data Sync Manager');

    const contextMenu = Menu.buildFromTemplate([
        { label: '表示 (Show)', click: () => mainWindow.show() },
        { label: '非表示 (Hide)', click: () => mainWindow.hide() },
        { type: 'separator' },
        { 
            label: '終了 (Quit)', 
            click: () => {
                app.isQuiting = true; // Flag to allow the 'close' handler to pass
                if (httpServer) {
                    httpServer.stop(); // Stop the HTTP server gracefully
                }
                app.quit();
            }
        }
    ]);
    
    // Left click: Toggle window visibility (System Tray Icon Operation)
    appTray.on('click', () => {
        mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show();
    });

    // Right click: Show context menu (System Tray Icon Operation)
    appTray.setContextMenu(contextMenu);
}


// --- IPC Communication (Renderer to Main) ---

ipcMain.handle('get-config', async () => {
    return ConfigManager.getConfig();
});

ipcMain.handle('save-config', async (event, newConfig) => {
    // Save new configuration
    ConfigManager.saveConfig(newConfig);
    // TODO: A real app should restart services if port/API credentials change.
    return true; 
});

ipcMain.handle('start-manual-sync', async () => {
    if (syncManager) {
        // Execute manual sync
        await syncManager.startSync(); 
        return { success: true, message: 'Synchronization completed.' };
    }
    return { success: false, message: 'Sync Manager not initialized.' };
});

ipcMain.handle('get-data-counts', async () => {
    // Helper function to fetch counts needed for the main dashboard
    // NOTE: This requires adding a method like dbManager.getTableCount(tableName)
    const dbManager = new DatabaseManager();
    await dbManager.connect();
    const counts = {
        shops: (await dbManager.all('SELECT COUNT(*) AS count FROM shops'))[0].count,
        shop_news: (await dbManager.all('SELECT COUNT(*) AS count FROM shop_news'))[0].count,
        event_news: (await dbManager.all('SELECT COUNT(*) AS count FROM event_news'))[0].count,
        specials: (await dbManager.all('SELECT COUNT(*) AS count FROM specials'))[0].count,
    };
    dbManager.close();
    return counts;
});


// --- Electron App Lifecycle ---

app.whenReady().then(async () => {
    console.log("Electron app ready.");
    
    // 1. Initialize core logic (DB, API, Sync Scheduling)
    await initializeCore();
    
    // 2. Create the main window and system tray
    createMainWindow();
    createTray();

    app.on('activate', () => {
        // On macOS it's common to re-create a window in the app when the dock icon is clicked
        if (BrowserWindow.getAllWindows().length === 0) createMainWindow();
    });
});

app.on('window-all-closed', () => {
    // Keep the app running in the background/tray when all windows are closed on non-macOS platforms
    if (process.platform !== 'darwin') {
        // Do nothing, app remains running in tray
    } else {
        // On macOS, quit the application completely if all windows are closed
        app.quit();
    }
});