package server

import (
	"bridge-ground/internal/logging"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AppInfo holds registration data for a connected external app.
type AppInfo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Version      string     `json:"version"`
	MallID       string     `json:"mallId"`
	Hostname     string     `json:"hostname"`
	IP           string     `json:"ip,omitempty"`
	RegisteredAt time.Time  `json:"registeredAt"`
	LastSeen     time.Time  `json:"lastSeen"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	Online       bool       `json:"online"`
	LogDir       string     `json:"logDir,omitempty"`
	LogPrefix    string     `json:"logPrefix,omitempty"`
}

// screenshotEntry holds the latest screenshot for one app.
type screenshotEntry struct {
	data        []byte
	at          time.Time
	contentType string
}

// wsEntry wraps a WebSocket connection with a write mutex.
type wsEntry struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// AppRegistry tracks registered external apps in memory.
type AppRegistry struct {
	mu          sync.Mutex
	apps        map[string]*AppInfo
	screenshots map[string]*screenshotEntry
	wsConns     map[string]*wsEntry
}

func newAppRegistry() *AppRegistry {
	return &AppRegistry{
		apps:        make(map[string]*AppInfo),
		screenshots: make(map[string]*screenshotEntry),
		wsConns:     make(map[string]*wsEntry),
	}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func randomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Register adds or updates an app. Re-registration by the same name+hostname
// updates the existing record instead of creating a duplicate.
// startedAt is the RFC3339 timestamp from the app itself (its process start time).
func (r *AppRegistry) Register(name, version, mallID, hostname, startedAt, logDir, logPrefix, ip string) AppInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	var parsedStartedAt *time.Time
	if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
		parsedStartedAt = &t
	} else if t, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
		parsedStartedAt = &t
	}

	for _, a := range r.apps {
		if a.Name == name && a.Hostname == hostname {
			a.Version = version
			a.MallID = mallID
			a.LastSeen = time.Now()
			if parsedStartedAt != nil {
				a.StartedAt = parsedStartedAt
			}
			if logDir != "" {
				a.LogDir = logDir
			}
			if logPrefix != "" {
				a.LogPrefix = logPrefix
			}
			if ip != "" {
				a.IP = ip
			}
			return *a
		}
	}
	a := &AppInfo{
		ID:           randomID(),
		Name:         name,
		Version:      version,
		MallID:       mallID,
		Hostname:     hostname,
		IP:           ip,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		StartedAt:    parsedStartedAt,
		LogDir:       logDir,
		LogPrefix:    logPrefix,
	}
	r.apps[a.ID] = a
	return *a
}

// Heartbeat updates the last-seen timestamp for an app.
func (r *AppRegistry) Heartbeat(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.apps[id]
	if !ok {
		return false
	}
	a.LastSeen = time.Now()
	return true
}

// List returns all registered apps with computed Online status.
func (r *AppRegistry) List() []AppInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-90 * time.Second)
	result := make([]AppInfo, 0, len(r.apps))
	for _, a := range r.apps {
		cp := *a
		cp.Online = a.LastSeen.After(cutoff)
		result = append(result, cp)
	}
	return result
}

// Get returns a single app by ID.
func (r *AppRegistry) Get(id string) (AppInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.apps[id]
	if !ok {
		return AppInfo{}, false
	}
	cutoff := time.Now().Add(-90 * time.Second)
	cp := *a
	cp.Online = a.LastSeen.After(cutoff)
	return cp, true
}

// StoreScreenshot saves screenshot bytes for an app.
func (r *AppRegistry) StoreScreenshot(id string, data []byte, contentType string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.apps[id]; !ok {
		return false
	}
	r.screenshots[id] = &screenshotEntry{data: data, at: time.Now(), contentType: contentType}
	return true
}

// GetScreenshot returns the latest screenshot for an app.
func (r *AppRegistry) GetScreenshot(id string) ([]byte, time.Time, string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.screenshots[id]
	if !ok {
		return nil, time.Time{}, "", false
	}
	return e.data, e.at, e.contentType, true
}

// SetWSConn registers a WebSocket connection for an app.
func (r *AppRegistry) SetWSConn(id string, entry *wsEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wsConns[id] = entry
}

// RemoveWSConn deregisters the WebSocket connection for an app.
func (r *AppRegistry) RemoveWSConn(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.wsConns, id)
}

// SendScreenshotRequest sends a screenshot_request message to an app via WebSocket.
func (r *AppRegistry) SendScreenshotRequest(id string) error {
	r.mu.Lock()
	entry := r.wsConns[id]
	r.mu.Unlock()

	if entry == nil {
		return fmt.Errorf("app not connected via WebSocket")
	}

	msg, _ := json.Marshal(map[string]string{"type": "screenshot_request"})
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.conn.WriteMessage(websocket.TextMessage, msg)
}

// GetAppLogs reads log entries for an app from its local log files.
func (r *AppRegistry) GetAppLogs(id, from, to string) []logging.LogEntry {
	r.mu.Lock()
	app, ok := r.apps[id]
	var logDir, logPrefix string
	if ok {
		logDir = app.LogDir
		logPrefix = app.LogPrefix
	}
	r.mu.Unlock()

	if !ok || logDir == "" || logPrefix == "" {
		return []logging.LogEntry{}
	}
	entries := logging.ReadAppLogsFromDir(logDir, logPrefix, from, to)
	if entries == nil {
		return []logging.LogEntry{}
	}
	return entries
}

// --- HTTP handlers ---

// handleAppsList handles GET /api/apps
func (s *Server) handleAppsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	list := s.Apps.List()
	if list == nil {
		list = []AppInfo{}
	}
	json.NewEncoder(w).Encode(list)
}

// handleAppsDetail handles all /api/apps/{...} sub-paths.
func (s *Server) handleAppsDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	sub := strings.TrimPrefix(r.URL.Path, "/api/apps/")

	// POST /api/apps/register
	if sub == "register" && r.Method == http.MethodPost {
		var req struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			MallID    string `json:"mallId"`
			Hostname  string `json:"hostname"`
			StartedAt string `json:"startedAt"`
			LogDir    string `json:"logDir"`
			LogPrefix string `json:"logPrefix"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Extract client IP; treat loopback as empty (portal manager will substitute)
		clientIP := r.RemoteAddr
		if host, _, err := net.SplitHostPort(clientIP); err == nil {
			clientIP = host
		}
		if clientIP == "127.0.0.1" || clientIP == "::1" {
			clientIP = ""
		}
		app := s.Apps.Register(req.Name, req.Version, req.MallID, req.Hostname, req.StartedAt, req.LogDir, req.LogPrefix, clientIP)
		json.NewEncoder(w).Encode(app)
		return
	}

	// POST /api/apps/{id}/heartbeat
	if strings.HasSuffix(sub, "/heartbeat") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(sub, "/heartbeat")
		if !s.Apps.Heartbeat(id) {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// POST /api/apps/{id}/screenshot/request — send capture command via WebSocket
	if strings.HasSuffix(sub, "/screenshot/request") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(sub, "/screenshot/request")
		if _, ok := s.Apps.Get(id); !ok {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}
		if err := s.Apps.SendScreenshotRequest(id); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// POST /api/apps/{id}/screenshot — app uploads screenshot bytes
	if strings.HasSuffix(sub, "/screenshot") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(sub, "/screenshot")
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			ct = "image/jpeg"
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, 20*1024*1024))
		if err != nil || len(data) == 0 {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if !s.Apps.StoreScreenshot(id, data, ct) {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}
		s.Broker.Broadcast("screenshot_ready", map[string]string{"appId": id})
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// GET /api/apps/{id}/screenshot — return stored screenshot
	if strings.HasSuffix(sub, "/screenshot") && r.Method == http.MethodGet {
		id := strings.TrimSuffix(sub, "/screenshot")
		data, _, ct, ok := s.Apps.GetScreenshot(id)
		if !ok || len(data) == 0 {
			http.Error(w, "no screenshot available", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	// GET /api/apps/{id}/screenshot/status
	if strings.HasSuffix(sub, "/screenshot/status") && r.Method == http.MethodGet {
		id := strings.TrimSuffix(sub, "/screenshot/status")
		_, at, _, ok := s.Apps.GetScreenshot(id)
		resp := map[string]interface{}{"ready": ok}
		if ok {
			resp["at"] = at.Format(time.RFC3339)
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	// GET /api/apps/{id}/logs?from=YYYY-MM-DD&to=YYYY-MM-DD
	if strings.HasSuffix(sub, "/logs") && r.Method == http.MethodGet {
		id := strings.TrimSuffix(sub, "/logs")
		today := time.Now().Format("2006-01-02")
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		if from == "" {
			from = today
		}
		if to == "" {
			to = today
		}
		entries := s.Apps.GetAppLogs(id, from, to)
		json.NewEncoder(w).Encode(entries)
		return
	}

	// GET /api/apps/{id}
	if r.Method == http.MethodGet {
		app, ok := s.Apps.Get(sub)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(app)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

// handleAppWS upgrades to WebSocket and maintains the connection for an app.
// Gido connects here after registration to receive screenshot_request commands.
func (s *Server) handleAppWS(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if _, ok := s.Apps.Get(id); !ok {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Warn("WS", fmt.Sprintf("WebSocket upgrade failed for app %s: %v", id, err))
		return
	}
	defer conn.Close()

	entry := &wsEntry{conn: conn}
	s.Apps.SetWSConn(id, entry)
	defer s.Apps.RemoveWSConn(id)

	logging.Info("WS", fmt.Sprintf("App %s connected via WebSocket", id))

	// Read loop: keeps the connection alive; ignores inbound messages.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	logging.Info("WS", fmt.Sprintf("App %s disconnected from WebSocket", id))
}
