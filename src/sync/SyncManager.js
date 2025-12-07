// src/sync/SyncManager.js
const ApiClient = require('./ApiClient');
const XmlParser = require('./XmlParser');
const DatabaseManager = require('../src/db/DatabaseManager');
const { getAppDataPath, downloadFile } = require('../utils/FileUtil');
const path = require('path');
const ConfigManager = require('../config/ConfigManager');

/**
 * @class SyncManager
 * Orchestrates the entire data synchronization process.
 */
class SyncManager {
    constructor() {
        this.config = ConfigManager.getConfig();
        this.apiClient = new ApiClient();
        this.xmlParser = new XmlParser();
        this.dbManager = new DatabaseManager();
        this.baseFileDir = path.join(getAppDataPath(), 'TTI', 'BridgeGround'); // Base path for local files
    }

    /**
     * @description Starts the full data synchronization process.
     * @returns {Promise<void>}
     */
    async startSync() {
        try {
            console.log("--- Starting Data Synchronization ---");
            
            // 1. Connect and initialize DB
            await this.dbManager.connect();
            await this.dbManager.initializeSchema();

            // 2. Sync all endpoints sequentially
            // Sync Shops data
            await this.syncEndpoint('/shoplist', this.xmlParser.processShopItem.bind(this.xmlParser), this.upsertShops.bind(this));
            
            // TODO: Add other endpoints: /eventnewslist, /shopnewslist, /speciallist, /genrelist
            
            console.log("--- Data Synchronization Completed Successfully ---");
        } catch (error) {
            console.error("Synchronization failed:", error.message);
        } finally {
            this.dbManager.close();
        }
    }
    
    /**
     * @description Generic method to handle fetching, parsing, and saving data for a single endpoint.
     * @param {string} endpoint - The API endpoint path.
     * @param {function} itemProcessor - Function to convert a parsed XML item to a DB object.
     * @param {function} upsertFunction - Function to save the processed items to the DB.
     * @returns {Promise<void>}
     */
    async syncEndpoint(endpoint, itemProcessor, upsertFunction) {
        console.log(`\nStarting sync for: ${endpoint}`);
        
        // 1. Fetch raw XML data
        const rawXml = await this.apiClient.fetchData(endpoint);
        
        // 2. Parse XML to structured object
        const parsedData = await this.xmlParser.parse(rawXml);
        
        // Assuming the list of items is under the 'item' key
        const rawItems = Array.isArray(parsedData.item) ? parsedData.item : (parsedData.item ? [parsedData.item] : []);
        console.log(`Found ${rawItems.length} items to process.`);

        const processedItems = [];
        for (const item of rawItems) {
            // 3. Process item (handle CDATA, identify remote URLs)
            const dbItem = itemProcessor(item, this.baseFileDir);
            
            // 4. Download files for the item
            await this.downloadMediaForShop(dbItem); // Custom download function
            
            processedItems.push(dbItem);
        }
        
        // 5. Upsert items to the database
        await upsertFunction(processedItems);
        console.log(`Finished sync for: ${endpoint}. ${processedItems.length} items saved.`);
    }

    /**
     * @description Handles downloading media files for a single Shop item.
     * Updates the local path fields in the item object.
     * @param {object} item - The processed DB item object (mutated).
     * @returns {Promise<void>}
     */
    async downloadMediaForShop(item) {
        const downloads = [];
        
        // Process Photo 1
        if (item.photo1_remote_url && item.photo1_local_path) {
            console.log(`  Downloading photo1 to ${item.photo1_local_path}`);
            downloads.push(downloadFile(item.photo1_remote_url, item.photo1_local_path));
        } else {
             // Ensure local path is null if remote is missing
             item.photo1_local_path = null; 
        }

        // Process Shop Logo
        if (item.shop_logo_remote_url && item.shop_logo_local_path) {
            console.log(`  Downloading shop_logo to ${item.shop_logo_local_path}`);
            downloads.push(downloadFile(item.shop_logo_remote_url, item.shop_logo_local_path));
        } else {
             // Ensure local path is null if remote is missing
             item.shop_logo_local_path = null;
        }

        try {
            await Promise.allSettled(downloads);
        } catch (error) {
             console.error(`Failed to download some media for shop ${item.shop_id}:`, error.message);
             // Fail silently for media download to allow data sync to continue
        }
    }

    /**
     * @description Saves/Updates the list of shop items to the database (Upsert logic).
     * @param {Array<object>} items - Array of processed shop data objects.
     * @returns {Promise<void>}
     */
    async upsertShops(items) {
        const update_date_field = 'update_date'; // Field to check for updates
        
        for (const item of items) {
            // 1. Check if the item already exists
            const existingRow = await this.dbManager.all('SELECT * FROM shops WHERE shop_id = ?', [item.shop_id]);
            
            if (existingRow.length > 0) {
                // Check if update_date is newer (or handle other update logic)
                // For simplicity here, we assume if it exists, we update it to ensure file paths are correct.
                
                // Construct UPDATE query
                const fields = Object.keys(item).filter(key => key !== 'shop_id' && !key.endsWith('_remote_url'));
                const setClauses = fields.map(field => `${field} = ?`).join(', ');
                
                // Get values in the correct order for the SQL parameters
                const values = fields.map(field => item[field]).concat([item.shop_id]);
                
                const sql = `UPDATE shops SET ${setClauses} WHERE shop_id = ?`;
                await this.dbManager.run(sql, values);
            } else {
                // 2. Insert new item
                const fields = Object.keys(item).filter(key => !key.endsWith('_remote_url'));
                const placeholders = fields.map(() => '?').join(', ');
                const values = fields.map(field => item[field]);

                const sql = `INSERT INTO shops (${fields.join(', ')}) VALUES (${placeholders})`;
                await this.dbManager.run(sql, values);
            }
        }
        console.log(`Successfully upserted ${items.length} shop records.`);
    }
    
    // TODO: Implement upsertEventNews, upsertShopNews, etc.
}

module.exports = SyncManager;