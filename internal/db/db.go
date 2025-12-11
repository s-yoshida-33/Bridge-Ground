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
	dir := filepath.Dir(m.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", m.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
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
	}
}

func (m *Manager) InitializeSchema() error {
	// Basic schema creation if not exists
	queries := []string{
		`CREATE TABLE IF NOT EXISTS shops (
			shop_id TEXT PRIMARY KEY,
			shop_name TEXT,
			shop_name_kana TEXT,
			shop_name_english TEXT,
			genre TEXT,
			genre_sub TEXT,
			tel TEXT,
			open_time TEXT,
			floor TEXT,
			photo1_remote_url TEXT,
			photo1_local_path TEXT,
			shop_logo_remote_url TEXT,
			shop_logo_local_path TEXT,
			update_date TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS event_news (
			event_id TEXT PRIMARY KEY,
			title TEXT,
			body TEXT,
			date_start TEXT,
			date_end TEXT,
			photo1_remote_url TEXT,
			photo1_local_path TEXT,
			update_date TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS shop_news (
			shop_news_id TEXT PRIMARY KEY,
			shop_id TEXT,
			title TEXT,
			body TEXT,
			photo1_remote_url TEXT,
			photo1_local_path TEXT,
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
	}

	for _, query := range queries {
		if _, err := m.Conn.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
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
