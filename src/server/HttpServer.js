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

        // Middleware to ensure JSON is pretty-printed (based on user request)
        this.app.set('json spaces', 2);
        
        this.setupRoutes();
    }

    /**
     * @description Sets up all the REST API endpoints defined in the specification.
     */
    setupRoutes() {
        const router = express.Router();

        // Base API endpoint path
        const API_BASE = '/api';

        /**
         * @description Generic handler to fetch all data from a specified table.
         * @param {string} tableName - The name of the table to query.
         */
        const createListEndpoint = (tableName) => {
            return async (req, res) => {
                try {
                    // Note: DB connection is not persistent in the current model. 
                    // Need to connect, query, and close per request for safety in this simple structure.
                    // For performance, the DB should ideally stay open, but we follow the simplest approach first.
                    await this.dbManager.connect();
                    const data = await this.dbManager.all(`SELECT * FROM ${tableName}`);
                    res.json(data);
                } catch (error) {
                    console.error(`Error fetching data from ${tableName}:`, error.message);
                    res.status(500).json({ error: `Failed to retrieve data from ${tableName}.` });
                } finally {
                    this.dbManager.close();
                }
            };
        };

        // --- API Endpoints ---
        
        // GET http://localhost:8080/api/shops (Shop list)
        router.get('/shops', createListEndpoint('shops'));

        // GET http://localhost:8080/api/shop-news (Shop news list)
        router.get('/shop-news', createListEndpoint('shop_news'));

        // GET http://localhost:8080/api/event-news (Event news list)
        router.get('/event-news', createListEndpoint('event_news'));

        // GET http://localhost:8080/api/specials (Feature list)
        router.get('/specials', createListEndpoint('specials'));
        
        // GET http://localhost:8080/api/genres (Genre list - added based on data structure)
        router.get('/genres', createListEndpoint('genres'));

        // TODO: The specification includes '/sales', which typically would be a filtered subset of 'specials'.
        // For simplicity, we can route it to specials for now or wait for clarification on filtering logic.
        // router.get('/sales', createListEndpoint('specials')); 

        // Apply the router to the base path
        this.app.use(API_BASE, router);
    }

    /**
     * @description Starts the HTTP server on the configured port.
     * @returns {void}
     */
    start() {
        const port = this.config.port;
        this.server = this.app.listen(port, () => {
            console.log(`HTTP Server running at http://localhost:${port}`);
            console.log(`Access the API at http://localhost:${port}/api/shops`);
        }).on('error', (err) => {
            if (err.code === 'EADDRINUSE') {
                console.error(`Port ${port} is already in use. Please check if another instance of BridgeGround is running, or change the port in settings.`);
            } else {
                console.error("HTTP Server startup error:", err.message);
            }
        });
    }

    /**
     * @description Stops the HTTP server.
     * @returns {void}
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