package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Manager struct {
	Conn *sql.DB
	Path string
}

type DataCounts struct {
	Shops     int `json:"shops"`
	ShopNews  int `json:"shop_news"`
	EventNews int `json:"event_news"`
	Specials  int `json:"specials"`
}

func NewManager() *Manager {
	// Construct path similar to Node.js version: %APPDATA%/TTI/BridgeGround/api/bridgeground.db
	appData, _ := os.UserConfigDir()
	dbDir := filepath.Join(appData, "TTI", "BridgeGround", "api")
	dbPath := filepath.Join(dbDir, "bridgeground.db")

	return &Manager{
		Path: dbPath,
	}
}

func (m *Manager) Connect() error {
	if m.Conn != nil {
		if err := m.Conn.Ping(); err == nil {
			return nil
		}
	}

	dir := filepath.Dir(m.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", m.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		// Log error but continue? Or fail?
		fmt.Printf("Warning: Failed to set WAL mode: %v\n", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	m.Conn = db
	return nil
}

func (m *Manager) Close() {
	if m.Conn != nil {
		m.Conn.Close()
		m.Conn = nil
	}
}

func (m *Manager) InitializeSchema() error {
	// Basic schema creation if not exists
	queries := []string{
		// `DROP TABLE IF EXISTS shops`, // Removed: This was causing full sync on every restart
		`CREATE TABLE IF NOT EXISTS shops (
			shop_id TEXT PRIMARY KEY,
			shop_name TEXT,
			shop_name_kana TEXT,
			shop_name_english TEXT,
			shop_name_china_cn TEXT,
			shop_name_china_tw TEXT,
			shop_name_korea TEXT,
			shop_name_france TEXT,
			shop_name_vietnam TEXT,
			shop_name_thai TEXT,
			abbr TEXT,
			web_status TEXT,
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
			open_day TEXT,
			close_flg TEXT,
			pub_start TEXT,
			pub_end TEXT,
			open_time TEXT,
			description TEXT,
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
			options TEXT,
			photo1 TEXT,
			photo1_remote_url TEXT,
			photo1_local_path TEXT,
			photo2 TEXT,
			photo2_remote_url TEXT,
			photo2_local_path TEXT,
			shop_logo TEXT,
			shop_logo_remote_url TEXT,
			shop_logo_local_path TEXT,
			photo1_thumb TEXT,
			photo1_thumb_150x150 TEXT,
			photo1_thumb_640x640 TEXT,
			photo1_thumb_w320 TEXT,
			photo1_thumb_w640 TEXT,
			photo1_thumb_w640_remote_url TEXT,
			photo1_thumb_w640_local_path TEXT,
			photo2_thumb TEXT,
			photo2_thumb_150x150 TEXT,
			photo2_thumb_640x640 TEXT,
			photo2_thumb_w320 TEXT,
			photo2_thumb_w640 TEXT,
			photo2_thumb_w640_remote_url TEXT,
			photo2_thumb_w640_local_path TEXT,
			shop_logo_thumb TEXT,
			shop_logo_thumb_150x150 TEXT,
			shop_logo_thumb_640x640 TEXT,
			shop_logo_thumb_640x640_remote_url TEXT,
			shop_logo_thumb_640x640_local_path TEXT,
			shop_logo_thumb_w320 TEXT,
			shop_logo_thumb_w640 TEXT,
			shop_logo_thumb_w640_remote_url TEXT,
			shop_logo_thumb_w640_local_path TEXT,
			update_date TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS event_news (
			event_id TEXT PRIMARY KEY,
			title TEXT,
			body TEXT,
			categories TEXT,
			date_start TEXT,
			date_end TEXT,
			display_end TEXT,
			venues TEXT,
			photo1 TEXT,
			photo1_remote_url TEXT,
			photo1_local_path TEXT,
			update_date TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS shop_news (
			shop_news_id TEXT PRIMARY KEY,
			shop_id TEXT,
			shop_name TEXT,
			shop_logo TEXT,
			shop_floors_name TEXT,
			title TEXT,
			body TEXT,
			categories TEXT,
			date_start TEXT,
			date_end TEXT,
			photo1 TEXT,
			photo1_remote_url TEXT,
			photo1_local_path TEXT,
			shop_logo_remote_url TEXT,
			shop_logo_local_path TEXT,
			update_date TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS genres (
			genre_id TEXT PRIMARY KEY,
			genre_name TEXT,
			genre_slug TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS specials (
			special_id TEXT PRIMARY KEY,
			special_title TEXT,
			title TEXT,
			special_sub_body TEXT,
			category_name TEXT,
			shop_id TEXT,
			shop_name TEXT,
			update_date TEXT,
			special_image_local_path TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS floors (
			floor_id TEXT PRIMARY KEY,
			floor_name TEXT,
			sort_order TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sync_meta (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
	}

	for _, query := range queries {
		if _, err := m.Conn.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	// Migrations: Try to add columns if they don't exist (ignore errors for existing columns)
	migrations := []string{
		"ALTER TABLE event_news ADD COLUMN categories TEXT",
		"ALTER TABLE event_news ADD COLUMN display_end TEXT",
		"ALTER TABLE event_news ADD COLUMN venues TEXT",
		"ALTER TABLE event_news ADD COLUMN photo1 TEXT",

		"ALTER TABLE shop_news ADD COLUMN shop_name TEXT",
		"ALTER TABLE shop_news ADD COLUMN shop_logo TEXT",
		"ALTER TABLE shop_news ADD COLUMN shop_logo_remote_url TEXT",
		"ALTER TABLE shop_news ADD COLUMN shop_logo_local_path TEXT",
		"ALTER TABLE shop_news ADD COLUMN shop_floors_name TEXT",
		"ALTER TABLE shop_news ADD COLUMN categories TEXT",
		"ALTER TABLE shop_news ADD COLUMN date_start TEXT",
		"ALTER TABLE shop_news ADD COLUMN date_end TEXT",
		"ALTER TABLE shop_news ADD COLUMN photo1 TEXT",

		// Thumbnail columns (existing)
		"ALTER TABLE shops ADD COLUMN photo1_thumb_w640 TEXT",
		"ALTER TABLE shops ADD COLUMN photo1_thumb_w640_remote_url TEXT",
		"ALTER TABLE shops ADD COLUMN photo1_thumb_w640_local_path TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb_w640 TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb_w640_remote_url TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb_w640_local_path TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_640x640 TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_640x640_remote_url TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_640x640_local_path TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_w640 TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_w640_remote_url TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_w640_local_path TEXT",

		// Full-field migration: multi-language names
		"ALTER TABLE shops ADD COLUMN shop_name_china_cn TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_china_tw TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_korea TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_france TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_vietnam TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_thai TEXT",
		"ALTER TABLE shops ADD COLUMN abbr TEXT",
		"ALTER TABLE shops ADD COLUMN web_status TEXT",

		// Full-field migration: multi-language genre memos
		"ALTER TABLE shops ADD COLUMN genre_memo_china_cn TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_china_tw TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_korea TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_france TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_vietnam TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_thai TEXT",

		// Full-field migration: additional shop info
		"ALTER TABLE shops ADD COLUMN tenant_code TEXT",
		"ALTER TABLE shops ADD COLUMN user_url TEXT",
		"ALTER TABLE shops ADD COLUMN floor TEXT",
		"ALTER TABLE shops ADD COLUMN open_year TEXT",
		"ALTER TABLE shops ADD COLUMN open_month TEXT",
		"ALTER TABLE shops ADD COLUMN open_day TEXT",
		"ALTER TABLE shops ADD COLUMN pub_start TEXT",
		"ALTER TABLE shops ADD COLUMN pub_end TEXT",
		"ALTER TABLE shops ADD COLUMN qr TEXT",
		"ALTER TABLE shops ADD COLUMN food_class TEXT",
		"ALTER TABLE shops ADD COLUMN seats TEXT",
		"ALTER TABLE shops ADD COLUMN smoking TEXT",
		"ALTER TABLE shops ADD COLUMN reservation TEXT",
		"ALTER TABLE shops ADD COLUMN lunch_menu TEXT",
		"ALTER TABLE shops ADD COLUMN dinner_menu TEXT",
		"ALTER TABLE shops ADD COLUMN take_out TEXT",
		"ALTER TABLE shops ADD COLUMN childrens_menu TEXT",
		"ALTER TABLE shops ADD COLUMN baby_seat TEXT",
		"ALTER TABLE shops ADD COLUMN alcohol TEXT",
		"ALTER TABLE shops ADD COLUMN options TEXT",

		// Full-field migration: additional thumbnail URL fields
		"ALTER TABLE shops ADD COLUMN photo1_thumb TEXT",
		"ALTER TABLE shops ADD COLUMN photo1_thumb_150x150 TEXT",
		"ALTER TABLE shops ADD COLUMN photo1_thumb_640x640 TEXT",
		"ALTER TABLE shops ADD COLUMN photo1_thumb_w320 TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb_150x150 TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb_640x640 TEXT",
		"ALTER TABLE shops ADD COLUMN photo2_thumb_w320 TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_150x150 TEXT",
		"ALTER TABLE shops ADD COLUMN shop_logo_thumb_w320 TEXT",
	}

	for _, query := range migrations {
		m.Conn.Exec(query) // Ignore error as column might exist
	}

	return nil
}

func (m *Manager) GetDataCounts() (*DataCounts, error) {
	// Ensure connection if not already connected
	if m.Conn == nil {
		if err := m.Connect(); err != nil {
			return nil, err
		}
		defer m.Close()
	}

	counts := &DataCounts{}

	// Ignore errors for individual counts, just return 0
	m.Conn.QueryRow("SELECT COUNT(*) FROM shops").Scan(&counts.Shops)
	m.Conn.QueryRow("SELECT COUNT(*) FROM shop_news").Scan(&counts.ShopNews)
	m.Conn.QueryRow("SELECT COUNT(*) FROM event_news").Scan(&counts.EventNews)
	m.Conn.QueryRow("SELECT COUNT(*) FROM specials").Scan(&counts.Specials)

	return counts, nil
}
