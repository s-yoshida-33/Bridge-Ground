// src/sync/SyncManager.js
const ApiClient = require('./ApiClient');
const XmlParser = require('./XmlParser');
const DatabaseManager = require('../db/DatabaseManager');
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
        this.BASE_FILE_DIR = path.join(getAppDataPath(), 'TTI', 'BridgeGround');
    }

    /**
     * @description Starts the full data synchronization process.
     */
    async startSync() {
        try {
            console.log("--- Starting Data Synchronization ---");
            
            // 1. Connect and initialize DB
            await this.dbManager.connect();
            await this.dbManager.initializeSchema();

            // 2. Sync all endpoints sequentially
            
            // Shops
            await this.syncEndpoint('/shoplist', this.xmlParser.processShopItem.bind(this.xmlParser), this.upsertShops.bind(this), this.downloadMediaForShop.bind(this));
            
            // Genres (No media)
            await this.syncEndpoint('/genrelist', this.xmlParser.processGenreItem.bind(this.xmlParser), this.upsertGenres.bind(this), async () => {});

            // Event News
            await this.syncEndpoint('/eventnewslist', this.xmlParser.processEventNewsItem.bind(this.xmlParser), this.upsertEventNews.bind(this), this.downloadMediaForEventNews.bind(this));
            
            // Shop News
            await this.syncEndpoint('/shopnewslist', this.xmlParser.processShopNewsItem.bind(this.xmlParser), this.upsertShopNews.bind(this), this.downloadMediaForShopNews.bind(this));

            // Specials/Features (Nested structure requires special sync)
            await this.syncSpecialEndpoint('/speciallist', this.xmlParser.processSpecialTitleItem.bind(this.xmlParser), this.upsertSpecials.bind(this));
            
            console.log("--- Data Synchronization Completed Successfully ---");
        } catch (error) {
            console.error("Synchronization failed:", error.message);
        } finally {
            this.dbManager.close();
        }
    }
    
    // --- Generic Sync and Upsert Logic ---

    async syncEndpoint(endpoint, itemProcessor, upsertFunction, mediaDownloader) {
        console.log(`\nStarting sync for: ${endpoint}`);
        
        const rawXml = await this.apiClient.fetchData(endpoint);
        const parsedData = await this.xmlParser.parse(rawXml);
        
        const rawItems = Array.isArray(parsedData.item) ? parsedData.item : (parsedData.item ? [parsedData.item] : []);
        const updateDateAll = parsedData.updateDateAll || null;
        const processedItems = [];

        for (const item of rawItems) {
            // Genre items need the overall updateDateAll passed to the processor
            const dbItem = (endpoint === '/genrelist') 
                ? itemProcessor(item, updateDateAll) 
                : itemProcessor(item); 
            
            await mediaDownloader(dbItem);
            processedItems.push(dbItem);
        }
        
        await upsertFunction(processedItems);
    }

    async syncSpecialEndpoint(endpoint, titleItemProcessor, upsertFunction) {
        console.log(`\nStarting complex sync for: ${endpoint}`);
        
        const rawXml = await this.apiClient.fetchData(endpoint);
        const parsedData = await this.xmlParser.parse(rawXml);
        
        const rawTitleItems = Array.isArray(parsedData.item) ? parsedData.item : (parsedData.item ? [parsedData.item] : []);
        const allSpecialItems = [];

        for (const titleItem of rawTitleItems) {
            const nestedItems = titleItemProcessor(titleItem); 
            
            for (const item of nestedItems) {
                 await this.downloadMediaForSpecial(item);
                 allSpecialItems.push(item);
            }
        }
        
        await upsertFunction(allSpecialItems);
    }
    
    // --- Media Download Helpers ---
    
    async downloadMediaForShop(item) {
        const downloads = [];
        if (item.photo1_remote_url && item.photo1_local_path) {
            downloads.push(downloadFile(item.photo1_remote_url, item.photo1_local_path));
        } else { item.photo1_local_path = null; }
        if (item.shop_logo_remote_url && item.shop_logo_local_path) {
            downloads.push(downloadFile(item.shop_logo_remote_url, item.shop_logo_local_path));
        } else { item.shop_logo_local_path = null; }

        await Promise.allSettled(downloads);
    }

    async downloadMediaForEventNews(item) {
        const downloads = [];
        if (item.photo1_remote_url && item.photo1_local_path) {
            downloads.push(downloadFile(item.photo1_remote_url, item.photo1_local_path));
        } else { item.photo1_local_path = null; }
        await Promise.allSettled(downloads);
    }

    async downloadMediaForShopNews(item) {
        const downloads = [];
        if (item.photo1_remote_url && item.photo1_local_path) {
            downloads.push(downloadFile(item.photo1_remote_url, item.photo1_local_path));
        } else { item.photo1_local_path = null; }
        await Promise.allSettled(downloads);
    }
    
    async downloadMediaForSpecial(item) {
        const downloads = [];
        if (item.special_image_remote_url && item.special_image_local_path) {
            downloads.push(downloadFile(item.special_image_remote_url, item.special_image_local_path));
        } else { item.special_image_local_path = null; }
        await Promise.allSettled(downloads);
    }

    // --- Upsert Logic ---
    
    async genericUpsert(tableName, idField, items) {
        for (const item of items) {
            const idValue = item[idField];
            if (!idValue) continue;

            // Remove remote URL fields before DB operation
            const itemToSave = Object.keys(item).reduce((acc, key) => {
                if (!key.endsWith('_remote_url')) {
                    acc[key.replace(/([A-Z])/g, "_$1").toLowerCase()] = item[key];
                }
                return acc;
            }, {});

            const fields = Object.keys(itemToSave).filter(key => key !== idField);
            const setClauses = fields.map(field => `${field} = ?`).join(', ');
            const values = fields.map(field => itemToSave[field]);
            
            const existingRow = await this.dbManager.all(`SELECT ${idField} FROM ${tableName} WHERE ${idField} = ?`, [idValue]);

            if (existingRow.length > 0) {
                // UPDATE
                const updateValues = values.concat([idValue]);
                const sql = `UPDATE ${tableName} SET ${setClauses} WHERE ${idField} = ?`;
                await this.dbManager.run(sql, updateValues);
            } else {
                // INSERT
                const insertFields = Object.keys(itemToSave);
                const placeholders = insertFields.map(() => '?').join(', ');
                const insertValues = insertFields.map(field => itemToSave[field]);

                const sql = `INSERT INTO ${tableName} (${insertFields.join(', ')}) VALUES (${placeholders})`;
                await this.dbManager.run(sql, insertValues);
            }
        }
    }

    async upsertShops(items) { await this.genericUpsert('shops', 'shop_id', items); }
    async upsertEventNews(items) { await this.genericUpsert('event_news', 'event_id', items); }
    async upsertShopNews(items) { await this.genericUpsert('shop_news', 'shop_news_id', items); }
    async upsertSpecials(items) { await this.genericUpsert('specials', 'special_id', items); }
    async upsertGenres(items) { await this.genericUpsert('genres', 'genre_id', items); }
}

module.exports = SyncManager;