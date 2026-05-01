package server

import (
	"bridge-ground/internal/logging"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AppInfo holds registration data for a connected external app.
type AppInfo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Version      string     `json:"version"`
	MallID       string     `json:"mallId"`
	Hostname     string     `json:"hostname"`
	RegisteredAt time.Time  `json:"registeredAt"`
	LastSeen     time.Time  `json:"lastSeen"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	Online       bool       `json:"online"`
}

// screenshotEntry holds the latest screenshot for one app.
type screenshotEntry struct {
	data []byte
	at   time.Time
}

// appLogBucket holds in-memory log entries per date for one app.
// Each date slice is capped at maxLogsPerDay entries (oldest trimmed first).
const maxLogsPerDay = 5000

type appLogBucket struct {
	mu     sync.Mutex
	byDate map[string][]logging.LogEntry // "YYYY-MM-DD" -> entries
}

func (b *appLogBucket) add(entry logging.LogEntry) {
	date := ""
	if len(entry.Timestamp) >= 10 {
		date = entry.Timestamp[:10]
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	slice := b.byDate[date]
	slice = append(slice, entry)
	if len(slice) > maxLogsPerDay {
		slice = slice[len(slice)-maxLogsPerDay:]
	}
	b.byDate[date] = slice
}

func (b *appLogBucket) getRange(from, to string) []logging.LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []logging.LogEntry
	for date, entries := range b.byDate {
		if date >= from && date <= to {
			result = append(result, entries...)
		}
	}
	return result
}

// AppRegistry tracks registered external apps in memory.
type AppRegistry struct {
	mu          sync.Mutex
	apps        map[string]*AppInfo
	screenshots map[string]*screenshotEntry
	appLogs     map[string]*appLogBucket
}

func newAppRegistry() *AppRegistry {
	return &AppRegistry{
		apps:        make(map[string]*AppInfo),
		screenshots: make(map[string]*screenshotEntry),
		appLogs:     make(map[string]*appLogBucket),
	}
}

func randomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Register adds or updates an app. Re-registration by the same name+hostname
// updates the existing record instead of creating a duplicate.
// startedAt is the RFC3339 timestamp from the app itself (its process start time).
func (r *AppRegistry) Register(name, version, mallID, hostname, startedAt string) AppInfo {
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
			return *a
		}
	}
	a := &AppInfo{
		ID:           randomID(),
		Name:         name,
		Version:      version,
		MallID:       mallID,
		Hostname:     hostname,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		StartedAt:    parsedStartedAt,
	}
	r.apps[a.ID] = a
	r.appLogs[a.ID] = &appLogBucket{byDate: make(map[string][]logging.LogEntry)}
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
func (r *AppRegistry) StoreScreenshot(id string, data []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.apps[id]; !ok {
		return false
	}
	r.screenshots[id] = &screenshotEntry{data: data, at: time.Now()}
	return true
}

// GetScreenshot returns the latest screenshot for an app.
func (r *AppRegistry) GetScreenshot(id string) ([]byte, time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.screenshots[id]
	if !ok {
		return nil, time.Time{}, false
	}
	return e.data, e.at, true
}

// AddLog appends a log entry for an app.
func (r *AppRegistry) AddLog(id string, entry logging.LogEntry) bool {
	r.mu.Lock()
	bucket, ok := r.appLogs[id]
	r.mu.Unlock()
	if !ok {
		return false
	}
	bucket.add(entry)
	return true
}

// GetAppLogs returns log entries for an app within the given date range.
func (r *AppRegistry) GetAppLogs(id, from, to string) []logging.LogEntry {
	r.mu.Lock()
	bucket, ok := r.appLogs[id]
	r.mu.Unlock()
	if !ok {
		return []logging.LogEntry{}
	}
	entries := bucket.getRange(from, to)
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
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		app := s.Apps.Register(req.Name, req.Version, req.MallID, req.Hostname, req.StartedAt)
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

	// POST /api/apps/{id}/screenshot/request
	if strings.HasSuffix(sub, "/screenshot/request") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(sub, "/screenshot/request")
		if _, ok := s.Apps.Get(id); !ok {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}
		s.Broker.Broadcast("screenshot_request", map[string]string{"appId": id})
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// POST /api/apps/{id}/screenshot — Gido uploads screenshot bytes
	if strings.HasSuffix(sub, "/screenshot") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(sub, "/screenshot")
		var buf []byte
		if r.ContentLength > 0 {
			buf = make([]byte, r.ContentLength)
			r.Body.Read(buf)
		}
		if !s.Apps.StoreScreenshot(id, buf) {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}
		s.Broker.Broadcast("screenshot_ready", map[string]string{"appId": id})
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	// GET /api/apps/{id}/screenshot — returns the stored screenshot image
	if strings.HasSuffix(sub, "/screenshot") && r.Method == http.MethodGet {
		id := strings.TrimSuffix(sub, "/screenshot")
		data, _, ok := s.Apps.GetScreenshot(id)
		if !ok || len(data) == 0 {
			http.Error(w, "no screenshot available", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	// GET /api/apps/{id}/screenshot/status
	if strings.HasSuffix(sub, "/screenshot/status") && r.Method == http.MethodGet {
		id := strings.TrimSuffix(sub, "/screenshot/status")
		_, at, ok := s.Apps.GetScreenshot(id)
		resp := map[string]interface{}{"ready": ok}
		if ok {
			resp["at"] = at.Format(time.RFC3339)
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	// POST /api/apps/{id}/logs — Gido uploads a single log entry
	if strings.HasSuffix(sub, "/logs") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(sub, "/logs")
		var entry logging.LogEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if entry.Timestamp == "" {
			entry.Timestamp = time.Now().Format("2006-01-02 15:04:05.000")
		}
		if !s.Apps.AddLog(id, entry) {
			http.Error(w, "app not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
