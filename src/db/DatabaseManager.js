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

    /**
     * Creates a temporary table with the same schema as the original table.
     * @param {string} originalTable The name of the existing table
     * @param {string} tempTable The name of the temporary table to create
     */
    async createTempTable(originalTable, tempTable) {
        if (!this.db) throw new Error("Database not connected.");
        
        // Get the schema of the original table
        const row = this.db.prepare(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`).get(originalTable);
        
        if (!row || !row.sql) {
            throw new Error(`Table ${originalTable} does not exist.`);
        }

        // Replace the table name in the CREATE statement
        // Handle "CREATE TABLE tableName" and "CREATE TABLE IF NOT EXISTS tableName"
        let createSql = row.sql;
        
        // Simple regex replacement might be risky if table name is a substring of column names, 
        // but for "CREATE TABLE [IF NOT EXISTS] name" it should be at the start.
        // Let's use a robust approach by replacing the first occurrence of the table name after "TABLE"
        
        // Regex to find "TABLE [IF NOT EXISTS] originalTable"
        const regex = new RegExp(`TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?("?${originalTable}"?)`, 'i');
        const match = createSql.match(regex);
        
        if (match) {
            // Replace the matched table name with temp table name
            // We need to be careful to reconstruct the string correctly
            const matchString = match[0]; // e.g. "TABLE shops" or "TABLE IF NOT EXISTS shops"
            const replacedString = matchString.replace(match[1], tempTable);
            createSql = createSql.replace(matchString, replacedString);
        } else {
             // Fallback: simple replace if regex fails (though it shouldn't for standard SQLite dump)
             createSql = createSql.replace(originalTable, tempTable);
        }

        this.db.exec(`DROP TABLE IF EXISTS ${tempTable}`);
        this.db.exec(createSql);
        console.log(`Created temporary table: ${tempTable}`);
    }

    /**
     * Swaps the temporary table with the original table using a transaction.
     * @param {string} tempTable 
     * @param {string} originalTable 
     */
    async swapTable(tempTable, originalTable) {
        if (!this.db) throw new Error("Database not connected.");

        const transaction = this.db.transaction(() => {
            // 1. Drop old backup if exists
            this.db.prepare(`DROP TABLE IF EXISTS ${originalTable}_old`).run();
            
            // 2. Rename current to backup (if it exists)
            // Check if original table exists first to avoid error
            const exists = this.db.prepare(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`).get(originalTable);
            if (exists) {
                this.db.prepare(`ALTER TABLE ${originalTable} RENAME TO ${originalTable}_old`).run();
            }
            
            // 3. Rename temp to original
            this.db.prepare(`ALTER TABLE ${tempTable} RENAME TO ${originalTable}`).run();
            
            // 4. Drop backup
            this.db.prepare(`DROP TABLE IF EXISTS ${originalTable}_old`).run();
        });

        try {
            transaction();
            console.log(`Swapped table ${tempTable} to ${originalTable}`);
        } catch (error) {
            console.error(`Failed to swap tables ${tempTable} -> ${originalTable}:`, error);
            throw error;
        }
    }
}

module.exports = DatabaseManager;
