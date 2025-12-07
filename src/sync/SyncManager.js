// src/sync/SyncManager.js
const ApiClient = require('./ApiClient');
const XmlParser = require('./XmlParser');
const DatabaseManager = require('../db/DatabaseManager');
const { getAppDataPath, downloadFile } = require('../utils/FileUtil');
const path = require('path');
const fs = require('fs');
const ConfigManager = require('../config/ConfigManager');

/**
 * @class SyncManager
 * Orchestrates the entire data synchronization process.
 */
class SyncManager {
    constructor(onProgress) {
        this.config = ConfigManager.getConfig();
        this.apiClient = new ApiClient();
        this.xmlParser = new XmlParser();
        this.dbManager = new DatabaseManager();
        this.BASE_FILE_DIR = path.join(getAppDataPath(), 'TTI', 'BridgeGround');
        this.onProgress = onProgress;
    }

    _reportProgress(mainState, subState) {
        if (this.onProgress) {
            this.onProgress({
                main: mainState, // { percentage, message }
                sub: subState    // { percentage, message }
            });
        }
    }

    /**
     * @description Starts the full data synchronization process.
     */
    async startSync() {
        const syncStats = {
            shops: 0,
            genres: 0,
            eventNews: 0,
            shopNews: 0,
            specials: 0
        };

        // Tasks definition for main progress
        const tasks = [
            { name: '店舗データ', key: 'shops', endpoint: '/shoplist', type: 'normal', tableName: 'shops' },
            { name: 'ジャンルデータ', key: 'genres', endpoint: '/genrelist', type: 'genre', tableName: 'genres' },
            { name: 'イベントニュース', key: 'eventNews', endpoint: '/eventnewslist', type: 'normal', tableName: 'event_news' },
            { name: 'ショップニュース', key: 'shopNews', endpoint: '/shopnewslist', type: 'normal', tableName: 'shop_news' },
            { name: '特集データ', key: 'specials', endpoint: '/speciallist', type: 'special', tableName: 'specials' }
        ];

        const totalTasks = tasks.length;

        try {
            console.log("--- Starting Data Synchronization ---");
            this._reportProgress({ percentage: 0, message: "同期を開始します..." }, { percentage: 0, message: "待機中..." });
            
            // 1. Connect and initialize DB
            await this.dbManager.connect();
            await this.dbManager.initializeSchema();

            // 2. Sync all endpoints sequentially
            for (let i = 0; i < totalTasks; i++) {
                const task = tasks[i];
                const mainPercentStart = (i / totalTasks) * 100;
                const mainPercentEnd = ((i + 1) / totalTasks) * 100;
                
                const updateMainProgress = (subPercent) => {
                    // Calculate main percentage based on sub task completion
                    // This makes the main bar move smoothly as sub-tasks complete
                    const currentMain = mainPercentStart + ((mainPercentEnd - mainPercentStart) * (subPercent / 100));
                    this._reportProgress(
                        { percentage: Math.round(currentMain), message: `${task.name}を同期中...` },
                        null // Sub progress is handled within the sync function, but we pass null here to not overwrite it if we were separating calls, but here we need to coordinate.
                        // Actually, _reportProgress expects full state.
                        // Let's pass a callback to the sync functions instead.
                    );
                };

                // Create a specific progress reporter for the sub-task
                const subProgressReporter = (percent, message) => {
                    const currentMain = mainPercentStart + ((mainPercentEnd - mainPercentStart) * (percent / 100));
                    this._reportProgress(
                        { percentage: Math.round(currentMain), message: `${task.name}を同期中...` },
                        { percentage: percent, message: message }
                    );
                };

                if (task.type === 'normal') {
                    let processor, upsert, downloader;
                    if (task.key === 'shops') {
                        processor = this.xmlParser.processShopItem.bind(this.xmlParser);
                        upsert = this.upsertShops.bind(this);
                        downloader = this.downloadMediaForShop.bind(this);
                    } else if (task.key === 'eventNews') {
                        processor = this.xmlParser.processEventNewsItem.bind(this.xmlParser);
                        upsert = this.upsertEventNews.bind(this);
                        downloader = this.downloadMediaForEventNews.bind(this);
                    } else if (task.key === 'shopNews') {
                        processor = this.xmlParser.processShopNewsItem.bind(this.xmlParser);
                        upsert = this.upsertShopNews.bind(this);
                        downloader = this.downloadMediaForShopNews.bind(this);
                    }

                    syncStats[task.key] = await this.syncEndpoint(
                        task.endpoint, 
                        task.tableName, 
                        processor, 
                        upsert, 
                        downloader, 
                        subProgressReporter
                    );

                } else if (task.type === 'genre') {
                    syncStats[task.key] = await this.syncEndpoint(
                        task.endpoint, 
                        task.tableName,
                        this.xmlParser.processGenreItem.bind(this.xmlParser), 
                        this.upsertGenres.bind(this), 
                        async () => {}, 
                        subProgressReporter
                    );
                } else if (task.type === 'special') {
                    syncStats[task.key] = await this.syncSpecialEndpoint(
                        task.endpoint, 
                        task.tableName,
                        this.xmlParser.processSpecialTitleItem.bind(this.xmlParser), 
                        this.upsertSpecials.bind(this),
                        subProgressReporter
                    );
                }
            }
            
            this._reportProgress(
                { percentage: 100, message: "同期完了", stats: syncStats }, 
                { percentage: 100, message: "全ての処理が完了しました" }
            );
            console.log("--- Data Synchronization Completed Successfully ---");
            console.log("Sync Stats:", syncStats);
            
            return syncStats;
        } catch (error) {
            this._reportProgress(
                { percentage: 0, message: "エラー停止" }, 
                { percentage: 0, message: `エラー: ${error.message}` }
            );
            console.error("Synchronization failed:", error.message);
            throw error;
        } finally {
            this.dbManager.close();
        }
    }

    /**
     * @description Saves the raw XML content to a file.
     */
    async _saveXmlToFile(endpoint, rawXml) {
        const fileName = endpoint.replace(/^\//, '') + '.xml';
        const xmlDir = path.join(this.BASE_FILE_DIR, 'xml');
        
        try {
            if (!fs.existsSync(xmlDir)) {
                await fs.promises.mkdir(xmlDir, { recursive: true });
            }
            await fs.promises.writeFile(path.join(xmlDir, fileName), rawXml, 'utf8');
            console.log(`Saved XML to: ${path.join(xmlDir, fileName)}`);
        } catch (error) {
            console.error(`Failed to save XML for ${endpoint}:`, error);
        }
    }
    
    // --- Generic Sync and Upsert Logic ---

    async syncEndpoint(endpoint, tableName, itemProcessor, upsertFunction, mediaDownloader, progressCallback) {
        console.log(`\nStarting sync for: ${endpoint}`);
        progressCallback(0, "データ取得中...");
        
        // Temp Table Setup
        const tempTableName = `${tableName}_temp`;
        let useTempTable = false;

        try {
            // Attempt to create a temporary table for safe sync
            await this.dbManager.createTempTable(tableName, tempTableName);
            useTempTable = true;
        } catch (error) {
            console.warn(`Could not create temporary table for ${tableName}. Proceeding with direct sync. Reason: ${error.message}`);
            // Fallback to direct sync if temp table creation fails (e.g., if main table doesn't exist yet, though initializeSchema should handle it)
        }
        
        const rawXml = await this.apiClient.fetchData(endpoint);
        
        // Save raw XML to file
        await this._saveXmlToFile(endpoint, rawXml);

        progressCallback(10, "解析中...");
        const parsedData = await this.xmlParser.parse(rawXml);
        
        const rawItems = Array.isArray(parsedData.item) ? parsedData.item : (parsedData.item ? [parsedData.item] : []);
        const updateDateAll = parsedData.updateDateAll || null;
        const processedItems = [];
        const totalItems = rawItems.length;

        if (totalItems === 0) {
            progressCallback(100, "データなし");
            // Clean up temp table if it was created
            if (useTempTable) {
                 // Even if no data, we might want to swap empty table? 
                 // User request implies "don't empty response during sync", but if result is empty, it should be empty.
                 // Assuming 0 items means "no data", we might just return here.
                 // However, if we want to reflect "0 items" in the DB, we should swap.
                 // Let's assume for now 0 items means nothing to update, keeping old data.
                 // If we want to clear data, we should probably swap.
                 // But sticking to "safe" approach: 0 items = no change.
            }
            return 0;
        }

        for (let i = 0; i < totalItems; i++) {
            const item = rawItems[i];
            
            // Calculate progress from 20% to 90%
            const percent = 20 + Math.round(((i) / totalItems) * 70);
            progressCallback(percent, `${i + 1}/${totalItems} 件処理中...`);

            const dbItem = (endpoint === '/genrelist') 
                ? itemProcessor(item, updateDateAll) 
                : itemProcessor(item); 
            
            await mediaDownloader(dbItem);
            processedItems.push(dbItem);
        }
        
        progressCallback(90, "データベース保存中...");
        
        const targetTable = useTempTable ? tempTableName : tableName;
        await upsertFunction(processedItems, targetTable);
        
        if (useTempTable) {
            progressCallback(95, "データ反映中...");
            await this.dbManager.swapTable(tempTableName, tableName);
        }
        
        progressCallback(100, "完了");
        return processedItems.length;
    }

    async syncSpecialEndpoint(endpoint, tableName, titleItemProcessor, upsertFunction, progressCallback) {
        console.log(`\nStarting complex sync for: ${endpoint}`);
        progressCallback(0, "データ取得中...");
        
        // Temp Table Setup
        const tempTableName = `${tableName}_temp`;
        let useTempTable = false;

        try {
            await this.dbManager.createTempTable(tableName, tempTableName);
            useTempTable = true;
        } catch (error) {
            console.warn(`Could not create temporary table for ${tableName}. Proceeding with direct sync. Reason: ${error.message}`);
        }

        const rawXml = await this.apiClient.fetchData(endpoint);
        
        // Save raw XML to file
        await this._saveXmlToFile(endpoint, rawXml);
        
        progressCallback(10, "解析中...");
        const parsedData = await this.xmlParser.parse(rawXml);
        
        const rawTitleItems = Array.isArray(parsedData.item) ? parsedData.item : (parsedData.item ? [parsedData.item] : []);
        const allSpecialItems = [];
        const totalTitles = rawTitleItems.length;

        if (totalTitles === 0) {
            progressCallback(100, "データなし");
            return 0;
        }

        for (let i = 0; i < totalTitles; i++) {
            const titleItem = rawTitleItems[i];
            
            // Calculate progress
            const percent = 20 + Math.round(((i) / totalTitles) * 70);
            progressCallback(percent, `特集グループ ${i + 1}/${totalTitles} 処理中...`);

            const nestedItems = titleItemProcessor(titleItem); 
            
            for (const item of nestedItems) {
                 await this.downloadMediaForSpecial(item);
                 allSpecialItems.push(item);
            }
        }
        
        progressCallback(90, "データベース保存中...");
        
        const targetTable = useTempTable ? tempTableName : tableName;
        await upsertFunction(allSpecialItems, targetTable);
        
        if (useTempTable) {
            progressCallback(95, "データ反映中...");
            await this.dbManager.swapTable(tempTableName, tableName);
        }
        
        progressCallback(100, "完了");
        return allSpecialItems.length;
    }
    
    // --- Media Download Helpers ---
    
    async downloadMediaForShop(item) {
        const downloads = [];
        const imagePrefixes = [
            'photo1', 
            'photo1_thumb', 'photo1_thumb150x150', 'photo1_thumb640x640', 'photo1_thumb_w320', 'photo1_thumb_w640',
            'photo2', 
            'photo2_thumb', 'photo2_thumb150x150', 'photo2_thumb640x640', 'photo2_thumb_w320', 'photo2_thumb_w640',
            'shop_logo', 
            'shop_logo_thumb', 'shop_logo_thumb150x150', 'shop_logo_thumb640x640', 'shop_logo_thumb_w320', 'shop_logo_thumb_w640'
        ];

        for (const prefix of imagePrefixes) {
            const remoteKey = `${prefix}_remote_url`;
            const localKey = `${prefix}_local_path`;

            if (item[remoteKey] && item[localKey]) {
                downloads.push(downloadFile(item[remoteKey], item[localKey]));
            } else {
                item[localKey] = null;
            }
        }

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
        if (!items || items.length === 0) return;
        
        // better-sqlite3 transaction support
        const runTransaction = this.dbManager.db.transaction((itemsToUpsert) => {
            for (const item of itemsToUpsert) {
                const idValue = item[idField];
                if (!idValue) continue;

                const itemToSave = Object.keys(item).reduce((acc, key) => {
                    if (!key.endsWith('_remote_url')) {
                        const dbKey = key.replace(/([A-Z])/g, "_$1").toLowerCase();
                        acc[dbKey] = item[key];
                    }
                    return acc;
                }, {});

                const fields = Object.keys(itemToSave).filter(key => key !== idField);
                const values = fields.map(field => itemToSave[field]);
                
                // Note: When using temp table, it starts empty, so everything is INSERT.
                // However, we still check existence to be safe or if genericUpsert is reused for incremental updates.
                const existingRow = this.dbManager.db.prepare(`SELECT ${idField} FROM ${tableName} WHERE ${idField} = ?`).get(idValue);

                if (existingRow) {
                    const setClauses = fields.map(field => `${field} = ?`).join(', ');
                    const updateValues = values.concat([idValue]);
                    const sql = `UPDATE ${tableName} SET ${setClauses} WHERE ${idField} = ?`;
                    this.dbManager.db.prepare(sql).run(...updateValues);
                } else {
                    const insertFields = Object.keys(itemToSave);
                    const placeholders = insertFields.map(() => '?').join(', ');
                    const insertValues = insertFields.map(field => itemToSave[field]);
                    const sql = `INSERT INTO ${tableName} (${insertFields.join(', ')}) VALUES (${placeholders})`;
                    this.dbManager.db.prepare(sql).run(...insertValues);
                }
            }
        });

        try {
            runTransaction(items);
        } catch (error) {
            console.error(`Error during batch upsert to ${tableName}:`, error);
            throw error;
        }
    }

    async upsertShops(items, tableName = 'shops') { await this.genericUpsert(tableName, 'shop_id', items); }
    async upsertEventNews(items, tableName = 'event_news') { await this.genericUpsert(tableName, 'event_id', items); }
    async upsertShopNews(items, tableName = 'shop_news') { await this.genericUpsert(tableName, 'shop_news_id', items); }
    async upsertSpecials(items, tableName = 'specials') { await this.genericUpsert(tableName, 'special_id', items); }
    async upsertGenres(items, tableName = 'genres') { await this.genericUpsert(tableName, 'genre_id', items); }
}

module.exports = SyncManager;
