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
        
        // 4. Handle initial and scheduled synchronization
        if (config.syncSettings.syncOnStartup) {
            console.log("Starting sync on startup...");
            await syncManager.startSync();
        }

        if (config.syncSettings.autoSyncEnabled) {
            const intervalMs = config.syncSettings.syncIntervalMinutes * 60 * 1000;
            console.log(`Auto sync enabled. Running every ${config.syncSettings.syncIntervalMinutes} minutes.`);
            // TODO: Implement actual scheduling logic (e.g., using setInterval)
            // Example: setInterval(() => syncManager.startSync(), intervalMs);
        }

    } catch (error) {
        console.error("Fatal error during application startup:", error.message);
        process.exit(1);
    }
}

main();