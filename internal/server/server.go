package server

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct {
	Config *config.Config
	DB     *db.Manager
}

func NewServer(cfg *config.Config, db *db.Manager) *Server {
	return &Server{
		Config: cfg,
		DB:     db,
	}
}

func (s *Server) Start() {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/shops", s.createListEndpoint("shops", "SELECT * FROM shops"))
	mux.HandleFunc("/api/shop-news", s.createListEndpoint("shop_news", "SELECT * FROM shop_news"))
	mux.HandleFunc("/api/event-news", s.createListEndpoint("event_news", "SELECT * FROM event_news"))
	mux.HandleFunc("/api/specials", s.createListEndpoint("specials", "SELECT * FROM specials"))
	mux.HandleFunc("/api/sales", s.createListEndpoint("specials", "SELECT * FROM specials")) // Sales maps to specials
	mux.HandleFunc("/api/genres", s.createListEndpoint("genres", "SELECT * FROM genres"))

	// Dashboard (Basic implementation)
	mux.HandleFunc("/", s.handleDashboard)

	// Bridge JS for UI integration
	mux.HandleFunc("/bridge.js", s.handleBridgeJS)

	port := s.Config.ServerSettings.Port
	addr := fmt.Sprintf(":%d", port)
	
	fmt.Printf("HTTP Server running at http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func (s *Server) createListEndpoint(tableName string, query string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.DB.Connect(); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer s.DB.Close()

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
		json.NewEncoder(w).Encode(result)
	}
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
