// src/main.js
const ConfigManager = require('./config/ConfigManager');
const SyncManager = require('./sync/SyncManager');
const HttpServer = require('./server/HttpServer');

/**
 * @description Main application entry point function.
 */
async function main() {
    console.log("--- Starting BridgeGround Application ---");

    try {
        // 1. Load Configuration
        const config = await ConfigManager.loadConfig();
        console.log(`Configuration loaded. API Base: ${config.apiSettings.baseUrl}`);

        // 2. Initialize and Start HTTP Server
        const server = new HttpServer();
        server.start();

        // 3. Initialize Sync Manager
        const syncManager = new SyncManager();
        
        // 4. Handle initial synchronization (Sync on Startup)
        if (config.syncSettings.syncOnStartup) {
            console.log("Starting sync on startup...");
            await syncManager.startSync();
        }

        // 5. Handle scheduled synchronization (Auto Sync)
        if (config.syncSettings.autoSyncEnabled) {
            const intervalMinutes = config.syncSettings.syncIntervalMinutes;
            const intervalMs = intervalMinutes * 60 * 1000;
            console.log(`Auto sync enabled. Running every ${intervalMinutes} minutes.`);
            
            // Start periodic synchronization
            setInterval(() => {
                console.log(`[Auto Sync] Triggered sync at interval of ${intervalMinutes} minutes.`);
                syncManager.startSync();
            }, intervalMs);
        }

        // TODO: Implement System Tray (for Electron/UI) to handle app shutdown and display status.

    } catch (error) {
        console.error("Fatal error during application startup:", error.message);
        process.exit(1);
    }
}

main();