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
        // Helper to extract image paths (remote URL and local path)
        const processImage = (tag) => {
            const relativePath = this.extractContent(item, tag);
            return {
                relative: relativePath,
                remote: this.getFullImageUrl(relativePath),
                local: this.getLocalSavePath(relativePath)
            };
        };

        const photo1 = processImage('photo1');
        const photo1Thumb = processImage('photo1Thumb');
        const photo1Thumb150 = processImage('photo1Thumb150x150');
        const photo1Thumb640 = processImage('photo1Thumb640x640');
        const photo1ThumbW320 = processImage('photo1ThumbW320');
        const photo1ThumbW640 = processImage('photo1ThumbW640');

        const photo2 = processImage('photo2');
        const photo2Thumb = processImage('photo2Thumb');
        const photo2Thumb150 = processImage('photo2Thumb150x150');
        const photo2Thumb640 = processImage('photo2Thumb640x640');
        const photo2ThumbW320 = processImage('photo2ThumbW320');
        const photo2ThumbW640 = processImage('photo2ThumbW640');

        const shopLogo = processImage('shopLogo');
        const shopLogoThumb = processImage('shopLogoThumb');
        const shopLogoThumb150 = processImage('shopLogoThumb150x150');
        const shopLogoThumb640 = processImage('shopLogoThumb640x640');
        const shopLogoThumbW320 = processImage('shopLogoThumbW320');
        const shopLogoThumbW640 = processImage('shopLogoThumbW640');
        
        // Explicitly map all fields to DB columns
        const shopData = {
            shop_id: item.shopId || null,
            shop_name: this.extractContent(item, 'shopName'),
            shop_name_kana: this.extractContent(item, 'shopNameKana'),
            shop_name_english: this.extractContent(item, 'shopNameEnglish'),
            shop_name_china_cn: this.extractContent(item, 'shopNameChinaCN'),
            shop_name_china_tw: this.extractContent(item, 'shopNameChinaTW'),
            shop_name_korea: this.extractContent(item, 'shopNameKorea'),
            shop_name_france: this.extractContent(item, 'shopNameFrance'),
            shop_name_vietnam: this.extractContent(item, 'shopNameVietnam'),
            shop_name_thai: this.extractContent(item, 'shopNameThai'),
            abbr: this.extractContent(item, 'abbr'),
            web_status: item.webStatus || null,
            searches: this.extractContent(item, 'searches'),
            genre: this.extractContent(item, 'genre'),
            genre_sub: this.extractContent(item, 'genreSub'),
            genre_sub_english: this.extractContent(item, 'genreSubEnglish'),
            genre_memo: this.extractContent(item, 'genreMemo'),
            genre_memo_english: this.extractContent(item, 'genreMemoEnglish'),
            genre_memo_china_cn: this.extractContent(item, 'genreMemoChinaCN'),
            genre_memo_china_tw: this.extractContent(item, 'genreMemoChinaTW'),
            genre_memo_korea: this.extractContent(item, 'genreMemoKorea'),
            genre_memo_france: this.extractContent(item, 'genreMemoFrance'),
            genre_memo_vietnam: this.extractContent(item, 'genreMemoVietnam'),
            genre_memo_thai: this.extractContent(item, 'genreMemoThai'),
            group_id: item.groupId || null,
            tenant_code: item.tenantCode || null,
            tel: this.extractContent(item, 'tel'),
            user_url: this.extractContent(item, 'userUrl'),
            floor: this.extractContent(item, 'floor'),
            floors: this.extractContent(item, 'floors'),
            area: this.extractContent(item, 'area'),
            area_sub: this.extractContent(item, 'areaSub'),
            number: this.extractContent(item, 'number'),
            open_year: item.openYear || null,
            open_month: item.openMonth || null,
            close_flg: item.closeFlg || null,
            pub_start: item.pubStart || null,
            pub_end: item.pubEnd || null,
            open_day: item.openDay || null,
            open_time: this.extractContent(item, 'openTime'),
            description: this.extractContent(item, 'description'),
            update_date: item.updateDate || null,

            // Media fields
            photo1: photo1.relative, photo1_remote_url: photo1.remote, photo1_local_path: photo1.local,
            photo1_thumb: photo1Thumb.relative, photo1_thumb_remote_url: photo1Thumb.remote, photo1_thumb_local_path: photo1Thumb.local,
            photo1_thumb150x150: photo1Thumb150.relative, photo1_thumb150x150_remote_url: photo1Thumb150.remote, photo1_thumb150x150_local_path: photo1Thumb150.local,
            photo1_thumb640x640: photo1Thumb640.relative, photo1_thumb640x640_remote_url: photo1Thumb640.remote, photo1_thumb640x640_local_path: photo1Thumb640.local,
            photo1_thumb_w320: photo1ThumbW320.relative, photo1_thumb_w320_remote_url: photo1ThumbW320.remote, photo1_thumb_w320_local_path: photo1ThumbW320.local,
            photo1_thumb_w640: photo1ThumbW640.relative, photo1_thumb_w640_remote_url: photo1ThumbW640.remote, photo1_thumb_w640_local_path: photo1ThumbW640.local,

            photo2: photo2.relative, photo2_remote_url: photo2.remote, photo2_local_path: photo2.local,
            photo2_thumb: photo2Thumb.relative, photo2_thumb_remote_url: photo2Thumb.remote, photo2_thumb_local_path: photo2Thumb.local,
            photo2_thumb150x150: photo2Thumb150.relative, photo2_thumb150x150_remote_url: photo2Thumb150.remote, photo2_thumb150x150_local_path: photo2Thumb150.local,
            photo2_thumb640x640: photo2Thumb640.relative, photo2_thumb640x640_remote_url: photo2Thumb640.remote, photo2_thumb640x640_local_path: photo2Thumb640.local,
            photo2_thumb_w320: photo2ThumbW320.relative, photo2_thumb_w320_remote_url: photo2ThumbW320.remote, photo2_thumb_w320_local_path: photo2ThumbW320.local,
            photo2_thumb_w640: photo2ThumbW640.relative, photo2_thumb_w640_remote_url: photo2ThumbW640.remote, photo2_thumb_w640_local_path: photo2ThumbW640.local,

            shop_logo: shopLogo.relative, shop_logo_remote_url: shopLogo.remote, shop_logo_local_path: shopLogo.local,
            shop_logo_thumb: shopLogoThumb.relative, shop_logo_thumb_remote_url: shopLogoThumb.remote, shop_logo_thumb_local_path: shopLogoThumb.local,
            shop_logo_thumb150x150: shopLogoThumb150.relative, shop_logo_thumb150x150_remote_url: shopLogoThumb150.remote, shop_logo_thumb150x150_local_path: shopLogoThumb150.local,
            shop_logo_thumb640x640: shopLogoThumb640.relative, shop_logo_thumb640x640_remote_url: shopLogoThumb640.remote, shop_logo_thumb640x640_local_path: shopLogoThumb640.local,
            shop_logo_thumb_w320: shopLogoThumbW320.relative, shop_logo_thumb_w320_remote_url: shopLogoThumbW320.remote, shop_logo_thumb_w320_local_path: shopLogoThumbW320.local,
            shop_logo_thumb_w640: shopLogoThumbW640.relative, shop_logo_thumb_w640_remote_url: shopLogoThumbW640.remote, shop_logo_thumb_w640_local_path: shopLogoThumbW640.local,

            qr: this.extractContent(item, 'qr'),
            food_class: this.extractContent(item, 'foodClass'),
            seats: this.extractContent(item, 'seats'),
            smoking: this.extractContent(item, 'smoking'),
            reservation: this.extractContent(item, 'reservation'),
            lunch_menu: this.extractContent(item, 'lunchMenu'),
            dinner_menu: this.extractContent(item, 'dinnerMenu'),
            take_out: this.extractContent(item, 'takeOut'),
            childrens_menu: this.extractContent(item, 'childrensMenu'),
            baby_seat: this.extractContent(item, 'babySeat'),
            alcohol: this.extractContent(item, 'alcohol'),
            options: this.extractContent(item, 'options')
        };

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
            date_end: item.dateEnd || null,
            pub_start: item.pubStart || null,
            pub_end: item.pubEnd || null,
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
            categories: this.extractContent(item, 'categories'),
            date_start: item.dateStart || null,
            date_end: item.dateEnd || null,
            pub_start: item.pubStart || null,
            pub_end: item.pubEnd || null,
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
                category_name: this.extractContent(item, 'categoryName'),
                shop_id: item.shopId || null,
                shop_name: this.extractContent(item, 'shopName'),
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