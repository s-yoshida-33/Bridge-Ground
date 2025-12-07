// src/sync/ApiClient.js
const axios = require('axios');
const ConfigManager = require('../config/ConfigManager');

/**
 * @class ApiClient
 * Handles all communication with the external Mall data API.
 */
class ApiClient {
    constructor() {
        this.config = ConfigManager.getConfig().apiSettings;
        this.client = axios.create({
            baseURL: this.config.baseUrl,
            timeout: 15000, // 15 seconds timeout for requests
            // Disable automatic throwing of errors for non-2xx status codes initially
            validateStatus: (status) => status >= 200 && status < 500, 
        });
        
        // Set Basic Auth header if credentials are provided
        if (this.config.username && this.config.password) {
            const token = Buffer.from(`${this.config.username}:${this.config.password}`).toString('base64');
            this.client.defaults.headers.common['Authorization'] = `Basic ${token}`;
        }
    }

    /**
     * @description Fetches data from a specified endpoint using GET method.
     * @param {string} endpoint - The API path (e.g., '/shoplist').
     * @returns {Promise<string>} The raw XML response string.
     * @throws {Error} If the API request fails or returns a non-200 status.
     */
    async fetchData(endpoint) {
        console.log(`Fetching data from: ${this.config.baseUrl}${endpoint}`);
        
        try {
            const response = await this.client.get(endpoint);

            if (response.status === 200) {
                // Returns the raw data which should be XML string
                return response.data;
            } else if (response.status === 401 || response.status === 403) {
                throw new Error(`Authentication failed for ${endpoint}. Status: ${response.status}`);
            } else {
                throw new Error(`API request failed for ${endpoint}. Status: ${response.status}. Data: ${response.data}`);
            }
        } catch (error) {
            console.error(`Error in ApiClient for ${endpoint}:`, error.message);
            throw new Error(`API connection failed or timed out: ${error.message}`);
        }
    }
    
    // TODO: Implement shoplist_search logic if needed, but current plan is to use local DB search.
}

module.exports = ApiClient;