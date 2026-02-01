package server

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct {
	Config *config.Config
	DB     *db.Manager
	Broker *EventBroker
}

func NewServer(cfg *config.Config, db *db.Manager) *Server {
	return &Server{
		Config: cfg,
		DB:     db,
		Broker: NewEventBroker(),
	}
}

func (s *Server) Start() {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/shops", s.handleShopList)
	mux.HandleFunc("/api/shop-news", s.handleShopNewsList)
	mux.HandleFunc("/api/event-news", s.handleEventNewsList)
	mux.HandleFunc("/api/specials", s.handleSpecialList)
	mux.HandleFunc("/api/sales", s.handleSpecialList) // Sales maps to specials
	mux.HandleFunc("/api/genres", s.handleGenreList)

	// SSE Endpoint
	mux.Handle("/api/events", s.Broker)

	// Dashboard (Basic implementation)
	mux.HandleFunc("/", s.handleDashboard)

	// Bridge JS for UI integration
	mux.HandleFunc("/bridge.js", s.handleBridgeJS)

	// Favicon
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=86400") // キャッシュを有効にする
		http.ServeFile(w, r, "src/assets/icon.ico")
	})
	// SVG Icon
	mux.HandleFunc("/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		http.ServeFile(w, r, "src/assets/icon.svg")
	})

	port := s.Config.ServerSettings.Port
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	fmt.Printf("HTTP Server running at http://localhost:%d\n", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

// BroadcastEvent sends an SSE event to all connected clients
func (s *Server) BroadcastEvent(eventType string, data interface{}) {
	s.Broker.Broadcast(eventType, data)
}

// FetchShopList retrieves the full list of shops from the database
func (s *Server) FetchShopList() ([]models.ShopItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT 
		shop_id, shop_name, shop_name_kana, shop_name_english, searches,
		genre, genre_sub, genre_sub_english, genre_memo, genre_memo_english,
		group_id, tel, floors, area, area_sub, number, close_flg,
		open_time, description, photo1, photo2, shop_logo, update_date,
		photo1_local_path, photo2_local_path, shop_logo_local_path
		FROM shops`

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var result []models.ShopItem

	for rows.Next() {
		var item models.ShopItem
		var (
			shopId, shopName, shopNameKana, shopNameEnglish, searches,
			genre, genreSub, genreSubEnglish, genreMemo, genreMemoEnglish,
			groupId, tel, floors, area, areaSub, number, closeFlg,
			openTime, description, photo1, photo2, shopLogo, updateDate,
			photo1LocalPath, photo2LocalPath, shopLogoLocalPath *string
		)

		if err := rows.Scan(
			&shopId, &shopName, &shopNameKana, &shopNameEnglish, &searches,
			&genre, &genreSub, &genreSubEnglish, &genreMemo, &genreMemoEnglish,
			&groupId, &tel, &floors, &area, &areaSub, &number, &closeFlg,
			&openTime, &description, &photo1, &photo2, &shopLogo, &updateDate,
			&photo1LocalPath, &photo2LocalPath, &shopLogoLocalPath,
		); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		s := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.ShopID = s(shopId)
		item.ShopName = s(shopName)
		item.ShopNameKana = s(shopNameKana)
		item.ShopNameEnglish = s(shopNameEnglish)
		item.Searches = s(searches)
		item.Genre = s(genre)
		item.GenreSub = s(genreSub)
		item.GenreSubEnglish = s(genreSubEnglish)
		item.GenreMemo = s(genreMemo)
		item.GenreMemoEnglish = s(genreMemoEnglish)
		item.GroupID = s(groupId)
		item.Tel = s(tel)
		item.Floors = s(floors)
		item.Area = s(area)
		item.AreaSub = s(areaSub)
		item.Number = s(number)
		item.CloseFlg = s(closeFlg)
		item.OpenTime = s(openTime)
		item.Description = s(description)
		item.Photo1 = s(photo1)
		item.Photo2 = s(photo2)
		item.ShopLogo = s(shopLogo)
		item.UpdateDate = s(updateDate)
		item.Photo1LocalPath = s(photo1LocalPath)
		item.Photo2LocalPath = s(photo2LocalPath)
		item.ShopLogoLocalPath = s(shopLogoLocalPath)

		result = append(result, item)
	}
	return result, nil
}

// FetchGenericList retrieves data for a generic table as a list of maps
func (s *Server) FetchGenericList(query string) ([]map[string]interface{}, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	var result []map[string]interface{}

	for rows.Next() {
		// Create a slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]

			// Handle SQLite types (often []byte or nil)
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			rowMap[col] = v
		}
		result = append(result, rowMap)
	}
	return result, nil
}

func (s *Server) createListEndpoint(tableName string, query string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := s.FetchGenericList(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false) // 追加: HTMLエスケープを無効化（文字化け対策の一つ）
		encoder.SetIndent("", "")    // 変更: インデントを無しにして1行にする（SSEと形式を合わせる）
		encoder.Encode(result)
	}
}

func (s *Server) handleShopList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchShopList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false) // 追加: HTMLエスケープを無効化
	encoder.SetIndent("", "  ")    // 変更: プリティプリントを有効化
	encoder.Encode(result)
}

// FetchEventNews retrieves event news from the database
func (s *Server) FetchEventNews() ([]models.EventNewsItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT 
		event_id, title, body, categories, date_start, date_end, display_end, venues,
		photo1, photo1_remote_url, photo1_local_path, update_date
		FROM event_news`

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var result []models.EventNewsItem

	for rows.Next() {
		var item models.EventNewsItem
		var (
			eventId, title, body, categories, dateStart, dateEnd, displayEnd, venues,
			photo1, photo1RemoteUrl, photo1LocalPath, updateDate *string
		)

		if err := rows.Scan(
			&eventId, &title, &body, &categories, &dateStart, &dateEnd, &displayEnd, &venues,
			&photo1, &photo1RemoteUrl, &photo1LocalPath, &updateDate,
		); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		s := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.EventID = s(eventId)
		item.Title = s(title)
		item.Body = s(body)
		item.Categories = s(categories)
		item.DateStart = s(dateStart)
		item.DateEnd = s(dateEnd)
		item.DisplayEnd = s(displayEnd)
		item.Venues = s(venues)
		item.Photo1 = s(photo1)
		item.Photo1RemoteURL = s(photo1RemoteUrl)
		item.Photo1LocalPath = s(photo1LocalPath)
		item.UpdateDate = s(updateDate)

		result = append(result, item)
	}
	return result, nil
}

// FetchShopNews retrieves shop news from the database
func (s *Server) FetchShopNews() ([]models.ShopNewsItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT 
		shop_news_id, shop_id, shop_name, shop_logo, shop_floors_name, title, body, categories,
		date_start, date_end, photo1, photo1_remote_url, photo1_local_path,
		shop_logo_remote_url, shop_logo_local_path, update_date
		FROM shop_news`

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var result []models.ShopNewsItem

	for rows.Next() {
		var item models.ShopNewsItem
		var (
			shopNewsId, shopId, shopName, shopLogo, shopFloorsName, title, body, categories,
			dateStart, dateEnd, photo1, photo1RemoteUrl, photo1LocalPath,
			shopLogoRemoteUrl, shopLogoLocalPath, updateDate *string
		)

		if err := rows.Scan(
			&shopNewsId, &shopId, &shopName, &shopLogo, &shopFloorsName, &title, &body, &categories,
			&dateStart, &dateEnd, &photo1, &photo1RemoteUrl, &photo1LocalPath,
			&shopLogoRemoteUrl, &shopLogoLocalPath, &updateDate,
		); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		s := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.ShopNewsID = s(shopNewsId)
		item.ShopID = s(shopId)
		item.ShopName = s(shopName)
		item.ShopLogo = s(shopLogo)
		item.ShopFloorsName = s(shopFloorsName)
		item.Title = s(title)
		item.Body = s(body)
		item.Categories = s(categories)
		item.DateStart = s(dateStart)
		item.DateEnd = s(dateEnd)
		item.Photo1 = s(photo1)
		item.Photo1RemoteURL = s(photo1RemoteUrl)
		item.Photo1LocalPath = s(photo1LocalPath)
		item.ShopLogoRemoteURL = s(shopLogoRemoteUrl)
		item.ShopLogoLocalPath = s(shopLogoLocalPath)
		item.UpdateDate = s(updateDate)

		result = append(result, item)
	}
	return result, nil
}

// FetchSpecials retrieves specials from the database
func (s *Server) FetchSpecials() ([]models.SpecialItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT 
		special_id, special_title, title, special_sub_body, category_name,
		shop_id, shop_name, update_date, special_image_local_path
		FROM specials`

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var result []models.SpecialItem

	for rows.Next() {
		var item models.SpecialItem
		var (
			specialId, specialTitle, title, specialSubBody, categoryName,
			shopId, shopName, updateDate, specialImageLocalPath *string
		)

		if err := rows.Scan(
			&specialId, &specialTitle, &title, &specialSubBody, &categoryName,
			&shopId, &shopName, &updateDate, &specialImageLocalPath,
		); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		s := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.SpecialID = s(specialId)
		item.SpecialTitle = s(specialTitle)
		item.Title = s(title)
		item.SpecialSubBody = s(specialSubBody)
		item.CategoryName = s(categoryName)
		item.ShopID = s(shopId)
		item.ShopName = s(shopName)
		item.UpdateDate = s(updateDate)
		item.SpecialImageLocalPath = s(specialImageLocalPath)

		result = append(result, item)
	}
	return result, nil
}

// FetchGenres retrieves genres from the database
func (s *Server) FetchGenres() ([]models.GenreItem, error) {
	if s.DB.Conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT genre_id, genre_name, genre_slug FROM genres`

	rows, err := s.DB.Conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var result []models.GenreItem

	for rows.Next() {
		var item models.GenreItem
		var genreId, genreName, genreSlug *string

		if err := rows.Scan(&genreId, &genreName, &genreSlug); err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		s := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.GenreID = s(genreId)
		item.GenreName = s(genreName)
		item.GenreSlug = s(genreSlug)

		result = append(result, item)
	}
	return result, nil
}

func (s *Server) handleEventNewsList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchEventNews()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.respondJSON(w, result)
}

func (s *Server) handleShopNewsList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchShopNews()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.respondJSON(w, result)
}

func (s *Server) handleSpecialList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchSpecials()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.respondJSON(w, result)
}

func (s *Server) handleGenreList(w http.ResponseWriter, r *http.Request) {
	result, err := s.FetchGenres()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.respondJSON(w, result)
}

func (s *Server) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// If looking for index.html explicitly or root, check if we should serve UI or Dashboard
	// The original JS version served a dashboard at / and the UI via Electron file loading.
	// But in Go version, we serve UI via lorca.
	// The main.go loads "http://localhost:port/index.html".
	// So if path is /index.html, serve static file.

	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		// Serve UI file if it exists in src/ui
		http.ServeFile(w, r, "src/ui/index.html")
		return
	}

	// Serve other static files from src/ui
	fs := http.FileServer(http.Dir("./src/ui"))
	fs.ServeHTTP(w, r)
}

func (s *Server) handleBridgeJS(w http.ResponseWriter, r *http.Request) {
	js := `
		console.log("Bridge JS Loaded");
		window.bridgeApi = {
			getConfig: async () => await window.go_getConfig(),
			getAppVersion: async () => await window.go_getAppVersion(),
			saveConfig: async (cfg) => await window.go_saveConfig(cfg),
			startManualSync: async () => await window.go_startManualSync(),
			getDataCounts: async () => await window.go_getDataCounts(),
			onSyncProgress: function(callback) {
				window._syncProgressCallback = callback;
			}
		};
		window.dispatchSyncProgress = function(data) {
			console.log("Sync Progress:", data);
			if (window._syncProgressCallback) {
				window._syncProgressCallback(data);
			}
		};
	`
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(js))
}




