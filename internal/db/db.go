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
	Sales     int `json:"sales"`
}

// AppRecord is the DB-persisted form of a registered external app.
type AppRecord struct {
	ID           string
	Name         string
	Version      string
	MallID       string
	Hostname     string
	IP           string
	RegisteredAt string
	LastSeen     string
	StartedAt    string
	LogDir       string
	LogPrefix    string
}

func NewManager() *Manager {
	appData, _ := os.UserConfigDir()
	dbDir := filepath.Join(appData, "TTI", "BridgeGround", "api")
	dbPath := filepath.Join(dbDir, "bridgeground.db")
	return &Manager{Path: dbPath}
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

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
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
	queries := []string{
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
			special_title_id TEXT,
			special_title TEXT,
			title TEXT,
			sub_title TEXT,
			category_id TEXT,
			category_name TEXT,
			special_sub_body TEXT,
			shop_id TEXT,
			shop_name TEXT,
			genre_memo TEXT,
			shop_logo TEXT,
			shop_logo_local_path TEXT,
			shop_floor_name TEXT,
			shop_floors_name TEXT,
			venue TEXT,
			pub_start TEXT,
			pub_end TEXT,
			update_date TEXT,
			special_image_remote_url TEXT,
			special_image_local_path TEXT,
			special_image2_local_path TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sales (
			sale_id TEXT PRIMARY KEY,
			sale_title_id TEXT,
			sale_title TEXT,
			sale_body TEXT,
			shop_id TEXT,
			shop_name TEXT,
			genre TEXT,
			genre_memo TEXT,
			shop_logo TEXT,
			shop_logo_remote_url TEXT,
			shop_logo_local_path TEXT,
			shop_floor_name TEXT,
			shop_floors_name TEXT,
			area TEXT,
			area_sub TEXT,
			pub_start TEXT,
			pub_end TEXT,
			update_date TEXT
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
		`CREATE TABLE IF NOT EXISTS registered_apps (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			version       TEXT,
			mall_id       TEXT,
			hostname      TEXT NOT NULL,
			ip            TEXT,
			registered_at TEXT NOT NULL,
			last_seen     TEXT NOT NULL,
			started_at    TEXT,
			log_dir       TEXT,
			log_prefix    TEXT
		)`,
	}

	for _, query := range queries {
		if _, err := m.Conn.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

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

		"ALTER TABLE shops ADD COLUMN shop_name_china_cn TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_china_tw TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_korea TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_france TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_vietnam TEXT",
		"ALTER TABLE shops ADD COLUMN shop_name_thai TEXT",
		"ALTER TABLE shops ADD COLUMN abbr TEXT",
		"ALTER TABLE shops ADD COLUMN web_status TEXT",

		"ALTER TABLE shops ADD COLUMN genre_memo_china_cn TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_china_tw TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_korea TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_france TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_vietnam TEXT",
		"ALTER TABLE shops ADD COLUMN genre_memo_thai TEXT",

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

		"ALTER TABLE specials ADD COLUMN special_title_id TEXT",
		"ALTER TABLE specials ADD COLUMN sub_title TEXT",
		"ALTER TABLE specials ADD COLUMN category_id TEXT",
		"ALTER TABLE specials ADD COLUMN genre_memo TEXT",
		"ALTER TABLE specials ADD COLUMN shop_logo TEXT",
		"ALTER TABLE specials ADD COLUMN shop_logo_local_path TEXT",
		"ALTER TABLE specials ADD COLUMN shop_floor_name TEXT",
		"ALTER TABLE specials ADD COLUMN shop_floors_name TEXT",
		"ALTER TABLE specials ADD COLUMN venue TEXT",
		"ALTER TABLE specials ADD COLUMN pub_start TEXT",
		"ALTER TABLE specials ADD COLUMN pub_end TEXT",
		"ALTER TABLE specials ADD COLUMN special_image_remote_url TEXT",
		"ALTER TABLE specials ADD COLUMN special_image2_local_path TEXT",

		"ALTER TABLE sales ADD COLUMN sale_title_image TEXT",
		"ALTER TABLE sales ADD COLUMN sale_title_image_local_path TEXT",
	}

	for _, query := range migrations {
		m.Conn.Exec(query)
	}

	return nil
}

func (m *Manager) GetDataCounts() (*DataCounts, error) {
	if m.Conn == nil {
		if err := m.Connect(); err != nil {
			return nil, err
		}
		defer m.Close()
	}

	counts := &DataCounts{}
	m.Conn.QueryRow("SELECT COUNT(*) FROM shops").Scan(&counts.Shops)
	m.Conn.QueryRow("SELECT COUNT(*) FROM shop_news").Scan(&counts.ShopNews)
	m.Conn.QueryRow("SELECT COUNT(*) FROM event_news").Scan(&counts.EventNews)
	m.Conn.QueryRow("SELECT COUNT(*) FROM specials").Scan(&counts.Specials)
	m.Conn.QueryRow("SELECT COUNT(*) FROM sales").Scan(&counts.Sales)
	return counts, nil
}

// UpsertApp inserts or replaces an app record in the DB.
func (m *Manager) UpsertApp(r AppRecord) error {
	if m.Conn == nil {
		return fmt.Errorf("db not connected")
	}
	_, err := m.Conn.Exec(
		`INSERT OR REPLACE INTO registered_apps
		(id, name, version, mall_id, hostname, ip, registered_at, last_seen, started_at, log_dir, log_prefix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Version, r.MallID, r.Hostname, r.IP,
		r.RegisteredAt, r.LastSeen, r.StartedAt, r.LogDir, r.LogPrefix,
	)
	return err
}

// UpdateAppLastSeen updates only the last_seen field for an app.
func (m *Manager) UpdateAppLastSeen(id, lastSeen string) error {
	if m.Conn == nil {
		return fmt.Errorf("db not connected")
	}
	_, err := m.Conn.Exec(`UPDATE registered_apps SET last_seen = ? WHERE id = ?`, lastSeen, id)
	return err
}

// LoadApps returns all persisted app records ordered by registration time.
func (m *Manager) LoadApps() ([]AppRecord, error) {
	if m.Conn == nil {
		return nil, fmt.Errorf("db not connected")
	}
	rows, err := m.Conn.Query(
		`SELECT id, name, version, mall_id, hostname, ip,
		        registered_at, last_seen, COALESCE(started_at,''), log_dir, log_prefix
		 FROM registered_apps ORDER BY registered_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []AppRecord
	for rows.Next() {
		var r AppRecord
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Version, &r.MallID, &r.Hostname, &r.IP,
			&r.RegisteredAt, &r.LastSeen, &r.StartedAt, &r.LogDir, &r.LogPrefix,
		); err != nil {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}
