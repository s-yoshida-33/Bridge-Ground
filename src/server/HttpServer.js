// src/server/HttpServer.js
const express = require('express');
const path = require('path');
const ConfigManager = require('../config/ConfigManager');
const DatabaseManager = require('../db/DatabaseManager');
const { getAppDataPath } = require('../utils/FileUtil');

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
        
        // Serve downloaded files statically
        const FILES_DIR = path.join(getAppDataPath(), 'TTI', 'BridgeGround', 'files');
        this.app.use('/files', express.static(FILES_DIR));
        
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

        // --- Dashboard Endpoint ---
        this.app.get('/', (req, res) => {
            const html = `
            <!DOCTYPE html>
            <html lang="ja">
            <head>
                <meta charset="UTF-8">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <title>BridgeGround API Dashboard</title>
                <style>
                    body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; padding: 40px; max-width: 800px; margin: 0 auto; line-height: 1.6; background-color: #f9f9f9; color: #333; }
                    h1 { color: #2c3e50; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
                    .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-top: 20px; }
                    ul { list-style-type: none; padding: 0; }
                    li { margin-bottom: 15px; padding-bottom: 15px; border-bottom: 1px solid #eee; }
                    li:last-child { border-bottom: none; }
                    a { text-decoration: none; color: #007bff; font-weight: bold; font-size: 1.1em; }
                    a:hover { text-decoration: underline; color: #0056b3; }
                    .note { color: #666; font-size: 0.9em; margin-top: 5px; }
                    .method { display: inline-block; background: #28a745; color: white; padding: 2px 6px; border-radius: 4px; font-size: 0.8em; margin-right: 8px; vertical-align: middle; }
                </style>
            </head>
            <body>
                <h1>BridgeGround Local API</h1>
                <div class="card">
                    <p>以下のエンドポイントから同期済みデータにアクセスできます。</p>
                    <ul>
                        <li>
                            <span class="method">GET</span>
                            <a href="/api/shops">/api/shops</a> 
                            <div class="note">店舗情報の全件リストを取得します。</div>
                        </li>
                        <li>
                            <span class="method">GET</span>
                            <a href="/api/shop-news">/api/shop-news</a> 
                            <div class="note">ショップニュースの全件リストを取得します。</div>
                        </li>
                        <li>
                            <span class="method">GET</span>
                            <a href="/api/event-news">/api/event-news</a> 
                            <div class="note">イベントニュースの全件リストを取得します。</div>
                        </li>
                        <li>
                            <span class="method">GET</span>
                            <a href="/api/specials">/api/specials</a> 
                            <div class="note">特集・セール情報の全件リストを取得します。(/api/sales と同等)</div>
                        </li>
                        <li>
                            <span class="method">GET</span>
                            <a href="/api/genres">/api/genres</a> 
                            <div class="note">ジャンルマスタのリストを取得します。</div>
                        </li>
                    </ul>
                </div>
            </body>
            </html>
            `;
            res.send(html);
        });

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