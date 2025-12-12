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
	mux.HandleFunc("/api/shop-news", s.createListEndpoint("shop_news", "SELECT * FROM shop_news"))
	mux.HandleFunc("/api/event-news", s.createListEndpoint("event_news", "SELECT * FROM event_news"))
	mux.HandleFunc("/api/specials", s.createListEndpoint("specials", "SELECT * FROM specials"))
	mux.HandleFunc("/api/sales", s.createListEndpoint("specials", "SELECT * FROM specials")) // Sales maps to specials
	mux.HandleFunc("/api/genres", s.createListEndpoint("genres", "SELECT * FROM genres"))

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

func (s *Server) createListEndpoint(tableName string, query string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.DB.Conn == nil {
			http.Error(w, "Database not connected", http.StatusInternalServerError)
			return
		}

		rows, err := s.DB.Conn.Query(query)
		if err != nil {
			http.Error(w, fmt.Sprintf("Query error: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			http.Error(w, "Failed to get columns", http.StatusInternalServerError)
			return
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

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		encoder.Encode(result)
	}
}

func (s *Server) handleShopList(w http.ResponseWriter, r *http.Request) {
	if s.DB.Conn == nil {
		http.Error(w, "Database not connected", http.StatusInternalServerError)
		return
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
		http.Error(w, fmt.Sprintf("Query error: %v", err), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(result)
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
