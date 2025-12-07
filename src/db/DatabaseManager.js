// src/db/DatabaseManager.js
const Database = require('better-sqlite3');
const fs = require('fs');
const path = require('path');
const { getAppDataPath } = require('../utils/FileUtil');

// Define the database file path
const APP_NAME = 'BridgeGround';
const DB_FILE_NAME = 'bridgeground.db';
const DB_DIR = path.join(getAppDataPath(), 'TTI', APP_NAME, 'api');
const DB_PATH = path.join(DB_DIR, DB_FILE_NAME);
const SETUP_SQL_PATH = path.join(__dirname, 'setup.sql');

/**
 * @class DatabaseManager
 * Manages the SQLite database connection and initialization using better-sqlite3.
 */
class DatabaseManager {
    constructor() {
        this.db = null;
    }

    /**
     * @description Connects to the SQLite database. Creates the directory if it doesn't exist.
     * @returns {Promise<void>}
     */
    async connect() {
        // Ensure the directory exists
        if (!fs.existsSync(DB_DIR)) {
            fs.mkdirSync(DB_DIR, { recursive: true });
        }

        try {
            // Open the database connection
            // better-sqlite3 opens the database synchronously
            this.db = new Database(DB_PATH);
            console.log('Connected to the SQLite database:', DB_PATH);
        } catch (err) {
            console.error("Could not connect to database:", err.message);
            throw err;
        }
    }

    /**
     * @description Closes the database connection.
     */
    close() {
        if (this.db) {
            try {
                this.db.close();
                console.log("Database connection closed.");
            } catch (err) {
                console.error("Error closing database:", err.message);
            }
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
            // better-sqlite3 supports executing multiple statements
            this.db.exec(sql);
            console.log("Database schema initialized successfully.");
        } catch (error) {
            console.error("Error initializing database schema:", error);
            throw error;
        }
    }
    
    /**
     * @description Executes a query that doesn't return data (e.g., INSERT, UPDATE, DELETE, CREATE).
     * @param {string} sql - The SQL statement to execute.
     * @param {Array<any>} params - The parameters to bind to the statement.
     * @returns {Promise<object>} A promise resolving to the result of the query execution.
     */
    async run(sql, params = []) {
        if (!this.db) {
            throw new Error("Database not connected.");
        }
        try {
            const stmt = this.db.prepare(sql);
            const info = stmt.run(...params);
            // info contains changes and lastInsertRowid
            return { id: info.lastInsertRowid, changes: info.changes };
        } catch (err) {
            console.error("Database run error:", err.message);
            throw err;
        }
    }

    /**
     * @description Executes a query and returns all result rows.
     * @param {string} sql - The SQL statement to execute.
     * @param {Array<any>} params - The parameters to bind to the statement.
     * @returns {Promise<Array<object>>} A promise resolving to an array of result rows.
     */
    async all(sql, params = []) {
        if (!this.db) {
            throw new Error("Database not connected.");
        }
        try {
            const stmt = this.db.prepare(sql);
            const rows = stmt.all(...params);
            return rows;
        } catch (err) {
            console.error("Database all error:", err.message);
            throw err;
        }
    }
    
    // TODO: Implement upsert logic for each table
}

module.exports = DatabaseManager;
