/*
 * @fileoverview SQL statements for initializing the BridgeGround database schema.
 * Defines tables based on the XML response structure.
 */

-- Table for Shops (from /shoplist)
CREATE TABLE IF NOT EXISTS shops (
    shop_id INTEGER PRIMARY KEY,
    shop_name TEXT NOT NULL,
    shop_name_kana TEXT,
    shop_name_english TEXT,
    shop_name_china_cn TEXT,
    shop_name_china_tw TEXT,
    shop_name_korea TEXT,
    shop_name_france TEXT,
    shop_name_vietnam TEXT,
    shop_name_thai TEXT,
    abbr TEXT,
    web_status INTEGER,
    searches TEXT,
    genre TEXT,
    genre_sub TEXT,
    genre_sub_english TEXT,
    genre_memo TEXT,
    genre_memo_english TEXT,
    genre_memo_china_cn TEXT,
    genre_memo_china_tw TEXT,
    genre_memo_korea TEXT,
    genre_memo_france TEXT,
    genre_memo_vietnam TEXT,
    genre_memo_thai TEXT,
    group_id TEXT,
    tenant_code TEXT,
    tel TEXT,
    user_url TEXT,
    floor TEXT,
    floors TEXT,
    area TEXT,
    area_sub TEXT,
    number TEXT,
    open_year TEXT,
    open_month TEXT,
    close_flg INTEGER,
    pub_start TEXT,
    pub_end TEXT,
    open_day TEXT,
    open_time TEXT,
    description TEXT,
    update_date TEXT,
    -- Images (Relative paths for API)
    photo1 TEXT,
    photo1_thumb TEXT,
    photo1_thumb150x150 TEXT,
    photo1_thumb640x640 TEXT,
    photo1_thumb_w320 TEXT,
    photo1_thumb_w640 TEXT,
    photo2 TEXT,
    photo2_thumb TEXT,
    photo2_thumb150x150 TEXT,
    photo2_thumb640x640 TEXT,
    photo2_thumb_w320 TEXT,
    photo2_thumb_w640 TEXT,
    shop_logo TEXT,
    shop_logo_thumb TEXT,
    shop_logo_thumb150x150 TEXT,
    shop_logo_thumb640x640 TEXT,
    shop_logo_thumb_w320 TEXT,
    shop_logo_thumb_w640 TEXT,
    
    -- Local paths for downloaded media files
    photo1_local_path TEXT,
    photo1_thumb_local_path TEXT,
    photo1_thumb150x150_local_path TEXT,
    photo1_thumb640x640_local_path TEXT,
    photo1_thumb_w320_local_path TEXT,
    photo1_thumb_w640_local_path TEXT,
    photo2_local_path TEXT,
    photo2_thumb_local_path TEXT,
    photo2_thumb150x150_local_path TEXT,
    photo2_thumb640x640_local_path TEXT,
    photo2_thumb_w320_local_path TEXT,
    photo2_thumb_w640_local_path TEXT,
    shop_logo_local_path TEXT,
    shop_logo_thumb_local_path TEXT,
    shop_logo_thumb150x150_local_path TEXT,
    shop_logo_thumb640x640_local_path TEXT,
    shop_logo_thumb_w320_local_path TEXT,
    shop_logo_thumb_w640_local_path TEXT,
    qr TEXT,
    food_class TEXT,
    seats TEXT,
    smoking TEXT,
    reservation TEXT,
    lunch_menu TEXT,
    dinner_menu TEXT,
    take_out TEXT,
    childrens_menu TEXT,
    baby_seat TEXT,
    alcohol TEXT,
    options TEXT
);

-- Table for Event News (from /eventnewslist)
CREATE TABLE IF NOT EXISTS event_news (
    event_id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT,
    categories TEXT,
    date_start TEXT,
    date_end TEXT,
    pub_start TEXT,
    pub_end TEXT,
    update_date TEXT,
    -- Local paths for downloaded media files
    photo1_local_path TEXT
);

-- Table for Shop News (from /shopnewslist)
CREATE TABLE IF NOT EXISTS shop_news (
    shop_news_id INTEGER PRIMARY KEY,
    shop_id INTEGER NOT NULL,
    shop_name TEXT,
    title TEXT NOT NULL,
    body TEXT,
    categories TEXT,
    date_start TEXT,
    date_end TEXT,
    pub_start TEXT,
    pub_end TEXT,
    update_date TEXT,
    -- Local paths for downloaded media files
    photo1_local_path TEXT
);

-- Table for Specials/Features (from /speciallist)
-- Stores the individual "special" items, linked back to the main title
CREATE TABLE IF NOT EXISTS specials (
    special_id INTEGER PRIMARY KEY,
    special_title TEXT,        -- The title from the parent <item type="specialTitle">
    title TEXT,                -- Individual item title
    special_sub_body TEXT,
    category_name TEXT,
    shop_id INTEGER,
    shop_name TEXT,
    update_date TEXT,
    -- Local paths for downloaded media files
    special_image_local_path TEXT
);

-- Table for Genres (from /genrelist)
CREATE TABLE IF NOT EXISTS genres (
    genre_id INTEGER PRIMARY KEY,
    genre_name TEXT NOT NULL,
    genre_slug TEXT,
    update_date_all TEXT -- The overall update date for genre list
);