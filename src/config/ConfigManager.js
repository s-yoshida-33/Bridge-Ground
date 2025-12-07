// src/config/ConfigManager.js
const fs = require('fs');
const path = require('path');
// Assuming FileUtil is implemented to find the AppData path
const { getAppDataPath, encrypt, decrypt } = require('../utils/FileUtil'); 

// Define the name and path for configuration file storage
const APP_NAME = 'BridgeGround';
const CONFIG_FILE_NAME = 'api_config.json';
const DEFAULT_CONFIG_PATH = path.join(__dirname, 'api_config_default.json');
const USER_CONFIG_DIR = path.join(getAppDataPath(), 'TTI', APP_NAME, 'api');
const USER_CONFIG_PATH = path.join(USER_CONFIG_DIR, CONFIG_FILE_NAME);

/**
 * @class ConfigManager
 * Manages reading, writing, and validating application configuration.
 */
class ConfigManager {
    constructor() {
        this.config = {};
    }

    /**
     * @description Loads the configuration from the user's app data directory or uses defaults.
     * @returns {Promise<object>} The loaded configuration object.
     */
    async loadConfig() {
        try {
            let rawData;
            if (fs.existsSync(USER_CONFIG_PATH)) {
                // Read user's saved configuration
                rawData = fs.readFileSync(USER_CONFIG_PATH, 'utf8');
                this.config = JSON.parse(rawData);
                
                // Decrypt password upon loading
                if (this.config.apiSettings.password) {
                    this.config.apiSettings.password = decrypt(this.config.apiSettings.password);
                }
            } else {
                // Use default configuration if user file is missing
                console.log("User config not found, loading defaults.");
                rawData = fs.readFileSync(DEFAULT_CONFIG_PATH, 'utf8');
                this.config = JSON.parse(rawData);
            }
            // TODO: Implement configuration validation (e.g., port range check)
            return this.config;
        } catch (error) {
            console.error("Error loading configuration:", error);
            // In a production app, error handling would prevent startup with bad config
            return this.config; 
        }
    }

    /**
     * @description Saves the configuration object to the user's app data directory.
     * @param {object} newConfig - The configuration object to save.
     * @returns {void}
     */
    saveConfig(newConfig) {
        // Encrypt password before saving to disk
        const configToSave = JSON.parse(JSON.stringify(newConfig)); // Deep copy
        if (configToSave.apiSettings.password) {
            configToSave.apiSettings.password = encrypt(configToSave.apiSettings.password);
        }

        if (!fs.existsSync(USER_CONFIG_DIR)) {
            fs.mkdirSync(USER_CONFIG_DIR, { recursive: true });
        }
        
        // Save the configuration in a readable, pretty-printed format (2 spaces)
        const jsonString = JSON.stringify(configToSave, null, 2);
        fs.writeFileSync(USER_CONFIG_PATH, jsonString, 'utf8');
        
        // Update the in-memory config with the non-encrypted version for app use
        this.config = newConfig;
    }

    /**
     * @description Gets the current configuration (with decrypted password).
     * @returns {object} The current configuration.
     */
    getConfig() {
        return this.config;
    }
}

module.exports = new ConfigManager();