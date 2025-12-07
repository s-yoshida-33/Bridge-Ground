// src/db/DatabaseManager.js
const sqlite3 = require('sqlite3').verbose();
const fs = require('fs');
const path = require('path');
const { getAppDataPath } = require('../utils/FileUtil');

// Define the database file path based on BridgeWebPopper's specification
const APP_NAME = 'BridgeGround';
const DB_FILE_NAME = 'bridgeground.db';
const DB_DIR = path.join(getAppDataPath(), 'TTI', APP_NAME, 'api');
const DB_PATH = path.join(DB_DIR, DB_FILE_NAME);
const SETUP_SQL_PATH = path.join(__dirname, 'setup.sql');

/**
 * @class DatabaseManager
 * Manages the SQLite database connection and initialization.
 */
class DatabaseManager {
    constructor() {
        this.db = null;
    }

    /**
     * @description Connects to the SQLite database. Creates the directory if it doesn't exist.
     * @returns {Promise<void>}
     */
    connect() {
        return new Promise((resolve, reject) => {
            // Ensure the directory exists
            if (!fs.existsSync(DB_DIR)) {
                fs.mkdirSync(DB_DIR, { recursive: true });
            }

            // Open the database connection
            this.db = new sqlite3.Database(DB_PATH, (err) => {
                if (err) {
                    console.error("Could not connect to database:", err.message);
                    return reject(err);
                }
                console.log('Connected to the SQLite database:', DB_PATH);
                resolve();
            });
        });
    }

    /**
     * @description Closes the database connection.
     */
    close() {
        if (this.db) {
            this.db.close((err) => {
                if (err) {
                    console.error("Error closing database:", err.message);
                } else {
                    console.log("Database connection closed.");
                }
            });
        }
    }

    /**
     * @description Executes the schema initialization script (setup.sql).
     * @returns {Promise<void>}
     */
    async initializeSchema() {
        if (!this.db) {
            throw new Error("Database not connected. Call connect() first.");
        }

        try {
            const sql = fs.readFileSync(SETUP_SQL_PATH, 'utf8');
            // SQLite driver runs sequential operations when using db.exec()
            return new Promise((resolve, reject) => {
                this.db.exec(sql, (err) => {
                    if (err) {
                        console.error("Error initializing database schema:", err.message);
                        return reject(err);
                    }
                    console.log("Database schema initialized successfully.");
                    resolve();
                });
            });
        } catch (error) {
            console.error("Error reading setup.sql file:", error);
            throw error;
        }
    }
    
    // TODO: Add methods for insert/update/select operations (e.g., run, get, all)
}

module.exports = DatabaseManager;