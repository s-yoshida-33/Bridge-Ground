package server

import (
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
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	MallID       string    `json:"mallId"`
	Hostname     string    `json:"hostname"`
	RegisteredAt time.Time `json:"registeredAt"`
	LastSeen     time.Time `json:"lastSeen"`
	Online       bool      `json:"online"`
}

// AppRegistry tracks registered external apps in memory.
type AppRegistry struct {
	mu   sync.Mutex
	apps map[string]*AppInfo
}

func newAppRegistry() *AppRegistry {
	return &AppRegistry{apps: make(map[string]*AppInfo)}
}

func randomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Register adds or updates an app. Re-registration by the same name+hostname
// updates the existing record instead of creating a duplicate.
func (r *AppRegistry) Register(name, version, mallID, hostname string) AppInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.apps {
		if a.Name == name && a.Hostname == hostname {
			a.Version = version
			a.MallID = mallID
			a.LastSeen = time.Now()
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

// handleAppsDetail handles all /api/apps/{...} sub-paths:
//
//	POST /api/apps/register          – register a new app
//	POST /api/apps/{id}/heartbeat    – update last-seen
//	GET  /api/apps/{id}              – get single app
func (s *Server) handleAppsDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	sub := strings.TrimPrefix(r.URL.Path, "/api/apps/")

	// POST /api/apps/register
	if sub == "register" && r.Method == http.MethodPost {
		var req struct {
			Name     string `json:"name"`
			Version  string `json:"version"`
			MallID   string `json:"mallId"`
			Hostname string `json:"hostname"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		app := s.Apps.Register(req.Name, req.Version, req.MallID, req.Hostname)
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
