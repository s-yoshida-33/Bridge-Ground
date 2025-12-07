// src/sync/XmlParser.js
const { parseStringPromise } = require('xml2js');
const path = require('path');

// Constants for file path reconstruction
const EXTERNAL_API_BASE_URL = ConfigManager.getConfig().apiSettings.baseUrl.replace(/\/api$/, '');

/**
 * @class XmlParser
 * Handles parsing raw XML strings into structured JSON objects suitable for DB storage.
 */
class XmlParser {
    /**
     * @description Converts raw XML data string to a structured JavaScript object (JSON).
     * @param {string} xmlString - The raw XML string from the API.
     * @returns {Promise<object>} The parsed JSON object.
     */
    async parse(xmlString) {
        // XML2JS options to simplify structure and handle CDATA
        const options = {
            explicitArray: false,        // Treat single children as objects, not arrays
            normalizeTags: false,        // Preserve case of tags
            trim: true,                  // Trim whitespace
            ignoreAttrs: true,           // Ignore XML attributes
            explicitCdata: true,         // Ensure CDATA content is handled
            // Custom processor to extract content from CDATA blocks
            charkey: 'content',
            childkey: 'children'
        };

        try {
            const result = await parseStringPromise(xmlString, options);
            // The top level is usually <data>
            return result.data;
        } catch (error) {
            console.error("XML Parsing error:", error.message);
            throw new Error("Failed to parse XML data from the API response.");
        }
    }

    /**
     * @description Processes a parsed shop item, handling CDATA and file paths.
     * @param {object} item - The parsed XML item object.
     * @param {string} appDataPath - The base path for saving files.
     * @returns {object} The standardized Shop object for DB insertion.
     */
    processShopItem(item, appDataPath) {
        // Helper function to safely extract content from a CDATA/text block
        const extractContent = (tag) => {
            const data = item[tag];
            if (typeof data === 'object' && data.content && data.content['#']) {
                // Handling data within CDATA section
                return data.content['#'];
            }
            if (typeof data === 'string') {
                return data;
            }
            return null;
        };

        // Helper function to construct the full image URL
        const getFullImageUrl = (relativeUrl) => {
            if (!relativeUrl) return null;
            // Assumes relative URL starts with /files/...
            return `${EXTERNAL_API_BASE_URL}${relativeUrl}`;
        };

        // Helper function to determine the local save path
        const getLocalSavePath = (relativeUrl, mediaType) => {
            if (!relativeUrl) return null;
            // Example: /files/shops/123/image.jpg -> %APPDATA%/TTI/BridgeGround/files/shops/123/image.jpg
            const fileName = path.basename(relativeUrl);
            const subDir = relativeUrl.split('/').slice(0, -1).join('/').replace(/^\/files\//, '');
            return path.join(appDataPath, 'files', subDir, fileName);
        };
        
        // Define the shop model fields and populate
        const shopData = {
            shop_id: item.shopId || null,
            shop_name: extractContent('shopName'),
            shop_name_kana: extractContent('shopNameKana'),
            tel: extractContent('tel'),
            user_url: extractContent('userUrl'),
            floor: extractContent('floor'),
            floors: extractContent('floors'),
            genre: extractContent('genre'),
            genre_sub: extractContent('genreSub'),
            description: extractContent('description'),
            web_status: item.webStatus || null,
            pub_start: item.pubStart || null,
            pub_end: item.pubEnd || null,
            update_date: item.updateDate || null
        };
        
        // File paths processing
        const photo1RelativePath = extractContent('photo1');
        const shopLogoRelativePath = extractContent('shopLogo');
        
        shopData.photo1_remote_url = getFullImageUrl(photo1RelativePath);
        shopData.shop_logo_remote_url = getFullImageUrl(shopLogoRelativePath);

        // Define local paths where files will be saved (Used by SyncManager)
        const baseFileDir = path.join(getAppDataPath(), 'TTI', 'BridgeGround');
        shopData.photo1_local_path = getLocalSavePath(photo1RelativePath, baseFileDir);
        shopData.shop_logo_local_path = getLocalSavePath(shopLogoRelativePath, baseFileDir);

        return shopData;
    }

    // TODO: Implement processEventNewsItem, processShopNewsItem, processSpecialItem, etc.
}

module.exports = XmlParser;