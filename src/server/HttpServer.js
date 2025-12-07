// src/server/HttpServer.js
const express = require('express');
const ConfigManager = require('../config/ConfigManager');
const DatabaseManager = require('../db/DatabaseManager');

/**
 * @class HttpServer
 * Provides the local REST API endpoints based on synchronized data.
 */
class HttpServer {
    constructor() {
        this.app = express();
        this.config = ConfigManager.getConfig().serverSettings;
        this.dbManager = new DatabaseManager();

        // Middleware to ensure JSON is pretty-printed (json spaces = 2)
        this.app.set('json spaces', 2);
        
        this.setupRoutes();
    }

    /**
     * @description Sets up all the REST API endpoints defined in the specification.
     */
    setupRoutes() {
        const router = express.Router();

        const API_BASE = '/api';

        const createListEndpoint = (tableName) => {
            return async (req, res) => {
                try {
                    await this.dbManager.connect();
                    const data = await this.dbManager.all(`SELECT * FROM ${tableName}`);
                    res.json(data);
                } catch (error) {
                    console.error(`Error fetching data from ${tableName}:`, error.message);
                    res.status(500).json({ error: `Failed to retrieve data from ${tableName}.` });
                } finally {
                    // Ensure DB connection is closed after each request
                    this.dbManager.close();
                }
            };
        };

        // --- API Endpoints (BridgeWebPopper Specification) ---
        
        router.get('/shops', createListEndpoint('shops'));             // ショップ一覧
        router.get('/shop-news', createListEndpoint('shop_news'));     // ショップニュース一覧
        router.get('/event-news', createListEndpoint('event_news'));   // イベントニュース一覧
        router.get('/specials', createListEndpoint('specials'));       // 特集一覧
        router.get('/sales', createListEndpoint('specials'));          // セール一覧 (特集合計にマッピング)
        
        // --- Additional Endpoints (Based on DB Schema) ---
        router.get('/genres', createListEndpoint('genres'));           // ジャンル一覧

        this.app.use(API_BASE, router);
    }

    /**
     * @description Starts the HTTP server on the configured port.
     */
    start() {
        const port = this.config.port;
        this.server = this.app.listen(port, () => {
            console.log(`HTTP Server running at http://localhost:${port}`);
        }).on('error', (err) => {
            if (err.code === 'EADDRINUSE') {
                console.error(`Port ${port} is already in use.`);
            } else {
                console.error("HTTP Server startup error:", err.message);
            }
        });
    }

    /**
     * @description Stops the HTTP server.
     */
    stop() {
        if (this.server) {
            this.server.close(() => {
                console.log("HTTP Server stopped.");
            });
        }
    }
}

module.exports = HttpServer;