/*
 * @fileoverview SQL statements for initializing the BridgeGround database schema.
 * Defines tables based on the XML response structure.
 */

-- Table for Shops (from /shoplist)
CREATE TABLE IF NOT EXISTS shops (
    shop_id INTEGER PRIMARY KEY,
    shop_name TEXT NOT NULL,
    shop_name_kana TEXT,
    tel TEXT,
    user_url TEXT,
    floor TEXT,
    floors TEXT,
    genre TEXT,
    genre_sub TEXT,
    description TEXT,
    web_status INTEGER,
    pub_start TEXT,
    pub_end TEXT,
    update_date TEXT,
    -- Local paths for downloaded media files
    photo1_local_path TEXT,
    shop_logo_local_path TEXT
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