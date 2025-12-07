// src/sync/XmlParser.js
const { parseStringPromise } = require('xml2js');
const path = require('path');
const ConfigManager = require('../config/ConfigManager');
const { getAppDataPath } = require('../utils/FileUtil');

/**
 * @class XmlParser
 * Handles parsing raw XML strings into structured JSON objects suitable for DB storage.
 */
class XmlParser {
    constructor() {
        // Base URL for image reconstruction. Assumes 'api' suffix is consistent.
        this.EXTERNAL_API_BASE_URL = ConfigManager.getConfig().apiSettings.baseUrl.replace(/\/api$/, '');
        this.BASE_FILE_DIR = path.join(getAppDataPath(), 'TTI', 'BridgeGround');
    }
    
    // --- Shared Helpers ---

    /**
     * @description Helper function to safely extract content from a CDATA/text block.
     */
    extractContent(item, tag) {
        const data = item[tag];
        if (typeof data === 'object' && data.content && data.content['#']) {
            // Handling data within CDATA section
            return data.content['#'];
        }
        if (typeof data === 'string') {
            return data;
        }
        return null;
    }

    /**
     * @description Helper function to construct the full image URL from a relative path.
     */
    getFullImageUrl(relativeUrl) {
        if (!relativeUrl) return null;
        return `${this.EXTERNAL_API_BASE_URL}${relativeUrl}`;
    }

    /**
     * @description Helper function to determine the local save path for a file.
     */
    getLocalSavePath(relativeUrl) {
        if (!relativeUrl) return null;
        // Example: /files/shops/123/image.jpg -> %APPDATA%/TTI/BridgeGround/files/shops/123/image.jpg
        const fileName = path.basename(relativeUrl);
        // Extracts the path after /files/ (e.g., shops/123)
        const subDir = relativeUrl.split('/').slice(0, -1).join('/').replace(/^\/files\//, '');
        return path.join(this.BASE_FILE_DIR, 'files', subDir, fileName);
    }
    
    // --- Core Parsing Method ---

    /**
     * @description Converts raw XML data string to a structured JavaScript object (JSON).
     */
    async parse(xmlString) {
        const options = {
            explicitArray: false,
            normalizeTags: false,
            trim: true,
            ignoreAttrs: true,
            explicitCdata: true,
            charkey: 'content',
            childkey: 'children'
        };
        try {
            const result = await parseStringPromise(xmlString, options);
            return result.data;
        } catch (error) {
            console.error("XML Parsing error:", error.message);
            throw new Error("Failed to parse XML data from the API response.");
        }
    }

    // --- Item Processors ---

    /**
     * @description Processes a parsed shop item.
     */
    processShopItem(item) {
        const photo1RelativePath = this.extractContent(item, 'photo1');
        const shopLogoRelativePath = this.extractContent(item, 'shopLogo');
        
        const shopData = {
            shop_id: item.shopId || null,
            shop_name: this.extractContent(item, 'shopName'),
            // ... (other text fields)
            update_date: item.updateDate || null,
            photo1_remote_url: this.getFullImageUrl(photo1RelativePath),
            shop_logo_remote_url: this.getFullImageUrl(shopLogoRelativePath),
            photo1_local_path: this.getLocalSavePath(photo1RelativePath),
            shop_logo_local_path: this.getLocalSavePath(shopLogoRelativePath),
        };
        // Populate all other fields dynamically to ensure completeness
        for (const key in item) {
            if (!shopData.hasOwnProperty(key)) {
                // Safely add other fields, converting nulls or CDATA/plain text
                shopData[key.toLowerCase()] = this.extractContent(item, key) || item[key] || null;
            }
        }
        return shopData;
    }

    /**
     * @description Processes a parsed Event News item.
     */
    processEventNewsItem(item) {
        const photo1RelativePath = this.extractContent(item, 'photo1');
        const eventData = {
            event_id: item.eventId || null,
            title: this.extractContent(item, 'title'),
            body: this.extractContent(item, 'body'),
            categories: this.extractContent(item, 'categories'),
            date_start: item.dateStart || null,
            // ... (other date fields)
            update_date: item.updateDate || null,
            photo1_remote_url: this.getFullImageUrl(photo1RelativePath),
            photo1_local_path: this.getLocalSavePath(photo1RelativePath),
        };
        return eventData;
    }

    /**
     * @description Processes a parsed Shop News item.
     */
    processShopNewsItem(item) {
        const photo1RelativePath = this.extractContent(item, 'photo1');
        const shopNewsData = {
            shop_news_id: item.shopNewsId || null,
            shop_id: item.shopId || null,
            shop_name: this.extractContent(item, 'shopName'),
            title: this.extractContent(item, 'title'),
            body: this.extractContent(item, 'body'),
            // ... (other fields)
            update_date: item.updateDate || null,
            photo1_remote_url: this.getFullImageUrl(photo1RelativePath),
            photo1_local_path: this.getLocalSavePath(photo1RelativePath),
        };
        return shopNewsData;
    }

    /**
     * @description Processes a Special/Feature Title item and extracts nested special items.
     */
    processSpecialTitleItem(parentItem) {
        const specialItems = [];
        const specialTitle = this.extractContent(parentItem, 'specialTitle');
        const updateDate = parentItem.updateDate || null;
        
        const rawSpecialItems = Array.isArray(parentItem.item) ? parentItem.item : (parentItem.item ? [parentItem.item] : []);

        for (const item of rawSpecialItems) {
            if (item.type !== 'special') continue;

            const imageRelativePath = this.extractContent(item, 'specialImage');
            const specialData = {
                special_id: item.specialId || null,
                special_title: specialTitle,
                title: this.extractContent(item, 'title'),
                special_sub_body: this.extractContent(item, 'specialSubBody'),
                shop_id: item.shopId || null,
                // ... (other text fields)
                update_date: updateDate,
                special_image_remote_url: this.getFullImageUrl(imageRelativePath),
                special_image_local_path: this.getLocalSavePath(imageRelativePath),
            };
            specialItems.push(specialData);
        }
        return specialItems;
    }

    /**
     * @description Processes a parsed Genre item.
     */
    processGenreItem(item, updateDateAll) {
        return {
            genre_id: item.genreId || null,
            genre_name: this.extractContent(item, 'genreName'),
            genre_slug: item.genreSlug || null,
            update_date_all: updateDateAll || null
        };
    }
}

module.exports = XmlParser;