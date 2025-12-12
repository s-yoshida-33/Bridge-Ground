package sync

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/models"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ProgressDetail struct {
	Percentage int            `json:"percentage"`
	Message    string         `json:"message"`
	Stats      map[string]int `json:"stats,omitempty"`
}

type SyncProgress struct {
	Main *ProgressDetail `json:"main,omitempty"`
	Sub  *ProgressDetail `json:"sub,omitempty"`
}

// Manager handles data synchronization
type Manager struct {
	Config           *config.Config
	DB               *db.Manager
	progressCallback func(SyncProgress)
}

// DownloadJob represents a task for the worker pool
type DownloadJob struct {
	RemoteURL string
	LocalPath string
}

func NewManager(cfg *config.Config, db *db.Manager) *Manager {
	return &Manager{
		Config: cfg,
		DB:     db,
	}
}

func (m *Manager) SetProgressCallback(cb func(SyncProgress)) {
	m.progressCallback = cb
}

func (m *Manager) notifyProgress(p SyncProgress) {
	if m.progressCallback != nil {
		m.progressCallback(p)
	}
}

func (m *Manager) getExistingUpdateDates(tableName, idCol string) (map[string]string, error) {
	query := fmt.Sprintf("SELECT %s, update_date FROM %s", idCol, tableName)
	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		// Table might not exist yet on first run
		return make(map[string]string), nil
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var id string
		var date interface{} // Use interface{} to handle potential NULLs or various types
		if err := rows.Scan(&id, &date); err != nil {
			continue
		}

		if date == nil {
			result[id] = ""
		} else {
			// Convert to string safely
			switch v := date.(type) {
			case []byte:
				result[id] = string(v)
			case string:
				result[id] = v
			default:
				result[id] = fmt.Sprintf("%v", v)
			}
		}
	}
	return result, nil
}

// StartSync executes the full synchronization process
func (m *Manager) StartSync() (bool, error) {
	fmt.Println("--- Starting Data Synchronization (Go) ---")

	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 0, Message: "開始中..."},
	})

	if m.DB.Conn == nil {
		if err := m.DB.Connect(); err != nil {
			m.notifyProgress(SyncProgress{
				Main: &ProgressDetail{Percentage: 0, Message: "DB接続エラー"},
			})
			return false, err
		}
	}

	if err := m.DB.InitializeSchema(); err != nil {
		return false, err
	}

	anyUpdated := false
	targets := m.Config.SyncSettings.SyncTargets
	if targets == nil {
		// Should verify default targets are set in config load, but fallback here
		targets = &config.SyncTargets{
			Shops:     true,
			ShopNews:  true,
			EventNews: true,
			Specials:  true,
			Genres:    true,
		}
	}

	// Track update counts for each category
	updateStats := make(map[string]int)

	// 1. Sync Shops
	if targets.Shops {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 10, Message: "店舗データを同期中..."},
		})
		count, err := m.syncShops()
		if err != nil {
			fmt.Printf("Error syncing shops: %v\n", err)
		}
		if count > 0 {
			anyUpdated = true
		}
		updateStats["shops"] = count
	} else {
		fmt.Println("Skipping shops sync (disabled in config)")
		updateStats["shops"] = 0
	}

	// 2. Sync Shop News
	if targets.ShopNews {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 30, Message: "ショップニュースを同期中..."},
		})
		count, err := m.syncShopNews()
		if err != nil {
			fmt.Printf("Error syncing shop news: %v\n", err)
		}
		if count > 0 {
			anyUpdated = true
		}
		updateStats["shopNews"] = count
	} else {
		fmt.Println("Skipping shop news sync (disabled in config)")
		updateStats["shopNews"] = 0
	}

	// 3. Sync Event News
	if targets.EventNews {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 50, Message: "イベントニュースを同期中..."},
		})
		count, err := m.syncEventNews()
		if err != nil {
			fmt.Printf("Error syncing event news: %v\n", err)
		}
		if count > 0 {
			anyUpdated = true
		}
		updateStats["eventNews"] = count
	} else {
		fmt.Println("Skipping event news sync (disabled in config)")
		updateStats["eventNews"] = 0
	}

	// 4. Sync Specials
	if targets.Specials {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 70, Message: "特集データを同期中..."},
		})
		count, err := m.syncSpecials()
		if err != nil {
			fmt.Printf("Error syncing specials: %v\n", err)
		}
		if count > 0 {
			anyUpdated = true
		}
		updateStats["specials"] = count
	} else {
		fmt.Println("Skipping specials sync (disabled in config)")
		updateStats["specials"] = 0
	}

	// 5. Sync Genres
	if targets.Genres {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 90, Message: "ジャンルデータを同期中..."},
		})
		count, err := m.syncGenres()
		if err != nil {
			fmt.Printf("Error syncing genres: %v\n", err)
		}
		if count > 0 {
			anyUpdated = true
		}
		// Genre updates are not tracked for "updated" flag currently (always 0)
	} else {
		fmt.Println("Skipping genres sync (disabled in config)")
	}

	fmt.Println("--- Synchronization Completed ---")

	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 100, Message: "同期完了", Stats: updateStats},
	})
	return anyUpdated, nil
}

// downloadWorker processes download jobs from the channel
func (m *Manager) downloadWorker(jobs <-chan DownloadJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		if err := m.downloadFile(job.RemoteURL, job.LocalPath); err != nil {
			// Log error but don't stop worker
			// fmt.Printf("[Worker] Failed to download %s: %v\n", job.RemoteURL, err)
		}
	}
}

func (m *Manager) downloadFile(url, destPath string) error {
	if url == "" || destPath == "" {
		return nil
	}

	// Skip if exists (optional optimization)
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (m *Manager) getBaseFileDir() string {
	appData, _ := os.UserConfigDir()
	return filepath.Join(appData, "TTI", "BridgeGround", "files")
}

func (m *Manager) resolveURL(relativePath string) string {
	base := strings.TrimRight(m.Config.APISettings.BaseURL, "/")
	cleanRel := strings.TrimLeft(relativePath, "/")
	return base + "/" + cleanRel
}

func (m *Manager) resolveLocalPath(baseDir, relativePath string) string {
	cleanRel := strings.TrimPrefix(relativePath, "/files") // Remove /files prefix if present as common pattern
	cleanRel = strings.TrimPrefix(cleanRel, "/")
	return filepath.Join(baseDir, cleanRel)
}

func (m *Manager) processDownloads(jobs []DownloadJob) {
	jobChan := make(chan DownloadJob, len(jobs))
	var wg sync.WaitGroup

	workerCount := 5
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go m.downloadWorker(jobChan, &wg)
	}

	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)
	wg.Wait()
}

// --- Sync Implementations ---

func (m *Manager) syncShops() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "shoplist"
	fmt.Printf("Fetching shops from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.ShopListResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return 0, err
	}

	existing, _ := m.getExistingUpdateDates("shops", "shop_id")
	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO shops (
		shop_id, shop_name, shop_name_kana, shop_name_english, searches, 
		genre, genre_sub, genre_sub_english, genre_memo, genre_memo_english, 
		group_id, tel, floors, area, area_sub, number, close_flg, 
		open_time, description, 
		photo1, photo2, shop_logo, update_date,
		photo1_remote_url, photo1_local_path, 
		photo2_remote_url, photo2_local_path, 
		shop_logo_remote_url, shop_logo_local_path
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		// Check update date
		oldDate, exists := existing[item.ShopID]
		// Determine if update is needed
		// Note: We use string comparison for update_date.
		// If the record doesn't exist, or the update_date is different, we count it as an update.
		// Also handle empty update_date which might cause constant updates if not handled
		isNew := !exists
		isUpdated := exists && oldDate != item.UpdateDate

		// If both old and new dates are empty, treat as no update needed to prevent loop
		if exists && oldDate == "" && item.UpdateDate == "" {
			isUpdated = false
		}

		// Additional check: Trim space to avoid formatting issues
		if exists && strings.TrimSpace(oldDate) == strings.TrimSpace(item.UpdateDate) {
			isUpdated = false
		}

		if isNew || isUpdated {
			// Debug logging (commented out for production)
			// fmt.Printf("[DEBUG] Shop update detected. ID: %s, Old: '%s', New: '%s'\n", item.ShopID, oldDate, item.UpdateDate)
			updateCount++
		}

		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		if item.Photo2 != "" {
			item.Photo2RemoteURL = m.resolveURL(item.Photo2)
			item.Photo2LocalPath = m.resolveLocalPath(baseFileDir, item.Photo2)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo2RemoteURL, item.Photo2LocalPath})
		}
		if item.ShopLogo != "" {
			item.ShopLogoRemoteURL = m.resolveURL(item.ShopLogo)
			item.ShopLogoLocalPath = m.resolveLocalPath(baseFileDir, item.ShopLogo)
			downloadJobs = append(downloadJobs, DownloadJob{item.ShopLogoRemoteURL, item.ShopLogoLocalPath})
		}
		stmt.Exec(
			item.ShopID, item.ShopName, item.ShopNameKana, item.ShopNameEnglish, item.Searches,
			item.Genre, item.GenreSub, item.GenreSubEnglish, item.GenreMemo, item.GenreMemoEnglish,
			item.GroupID, item.Tel, item.Floors, item.Area, item.AreaSub, item.Number, item.CloseFlg,
			item.OpenTime, item.Description,
			item.Photo1, item.Photo2, item.ShopLogo, item.UpdateDate,
			item.Photo1RemoteURL, item.Photo1LocalPath,
			item.Photo2RemoteURL, item.Photo2LocalPath,
			item.ShopLogoRemoteURL, item.ShopLogoLocalPath,
		)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	m.processDownloads(downloadJobs)
	return updateCount, nil
}

func (m *Manager) syncShopNews() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "shopnewslist"
	fmt.Printf("Fetching shop news from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.ShopNewsResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return 0, err
	}

	existing, _ := m.getExistingUpdateDates("shop_news", "shop_news_id")
	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO shop_news (shop_news_id, shop_id, title, body, photo1_remote_url, photo1_local_path, update_date) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		// Check update date
		oldDate, exists := existing[item.ShopNewsID]
		if !exists || (oldDate != item.UpdateDate && item.UpdateDate != "" && strings.TrimSpace(oldDate) != strings.TrimSpace(item.UpdateDate)) {
			// fmt.Printf("[DEBUG] ShopNews update detected. ID: %s, Old: '%s', New: '%s'\n", item.ShopNewsID, oldDate, item.UpdateDate)
			updateCount++
		}

		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		stmt.Exec(item.ShopNewsID, item.ShopID, item.Title, item.Body, item.Photo1RemoteURL, item.Photo1LocalPath, item.UpdateDate)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	m.processDownloads(downloadJobs)
	return updateCount, nil
}

func (m *Manager) syncEventNews() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "eventnewslist"
	fmt.Printf("Fetching event news from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.EventNewsResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return 0, err
	}

	existing, _ := m.getExistingUpdateDates("event_news", "event_id")
	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO event_news (event_id, title, body, date_start, date_end, photo1_remote_url, photo1_local_path, update_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		// Check update date
		oldDate, exists := existing[item.EventID]
		if !exists || (oldDate != item.UpdateDate && item.UpdateDate != "" && strings.TrimSpace(oldDate) != strings.TrimSpace(item.UpdateDate)) {
			// fmt.Printf("[DEBUG] EventNews update detected. ID: %s, Old: '%s', New: '%s'\n", item.EventID, oldDate, item.UpdateDate)
			updateCount++
		}

		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		stmt.Exec(item.EventID, item.Title, item.Body, item.DateStart, item.DateEnd, item.Photo1RemoteURL, item.Photo1LocalPath, item.UpdateDate)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	m.processDownloads(downloadJobs)
	return updateCount, nil
}

func (m *Manager) syncSpecials() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "speciallist"
	fmt.Printf("Fetching specials from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.SpecialListResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return 0, err
	}

	existing, _ := m.getExistingUpdateDates("specials", "special_id")
	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO specials (special_id, special_title, title, special_sub_body, category_name, shop_id, shop_name, update_date, special_image_local_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	// Specials are nested in specialTitle items
	for _, parent := range resp.Items {
		// Iterate over sub-items
		for _, item := range parent.Items {
			if item.Type != "special" {
				continue
			}

			// Use parent properties where appropriate
			item.SpecialTitle = parent.SpecialTitle
			item.UpdateDate = parent.UpdateDate

			// Check update date
			oldDate, exists := existing[item.SpecialID]
			if !exists || (oldDate != item.UpdateDate && item.UpdateDate != "" && strings.TrimSpace(oldDate) != strings.TrimSpace(item.UpdateDate)) {
				// fmt.Printf("[DEBUG] Special update detected. ID: %s, Old: '%s', New: '%s'\n", item.SpecialID, oldDate, item.UpdateDate)
				updateCount++
			}

			if item.SpecialImage != "" {
				item.SpecialImageRemoteURL = m.resolveURL(item.SpecialImage)
				item.SpecialImageLocalPath = m.resolveLocalPath(baseFileDir, item.SpecialImage)
				downloadJobs = append(downloadJobs, DownloadJob{item.SpecialImageRemoteURL, item.SpecialImageLocalPath})
			}
			stmt.Exec(item.SpecialID, item.SpecialTitle, item.Title, item.SpecialSubBody, item.CategoryName, item.ShopID, item.ShopName, item.UpdateDate, item.SpecialImageLocalPath)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	m.processDownloads(downloadJobs)
	return updateCount, nil
}

func (m *Manager) syncGenres() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "genrelist"
	fmt.Printf("Fetching genres from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.GenreListResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return 0, err
	}

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO genres (genre_id, genre_name, genre_slug) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	// Genres don't have update_date, so we treat all as processed but not necessarily "updated".
	// However, since we don't track changes for genres, we can either return 0 or the count of items.
	// Returning 0 maintains the behavior that genres don't trigger "new updates" notifications usually.
	for _, item := range resp.Items {
		stmt.Exec(item.GenreID, item.GenreName, item.GenreSlug)
	}
	// Genres don't have update_date, assume false or maybe we should hash?
	// For now, returning false as they are master data and rarely change
	return 0, tx.Commit()
}

func (m *Manager) fetchXML(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if m.Config.APISettings.Username != "" {
		req.SetBasicAuth(m.Config.APISettings.Username, m.Config.APISettings.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Save XML dump
	go m.saveXMLDump(url, data)

	return data, nil
}

func (m *Manager) saveXMLDump(endpoint string, data []byte) {
	// Get user config directory (AppData/Roaming on Windows)
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Failed to get config dir for XML dump: %v\n", err)
		return
	}

	// Target directory: .../TTI/BridgeGround/xml
	dir := filepath.Join(configDir, "TTI", "BridgeGround", "xml")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Failed to create XML dump directory: %v\n", err)
		return
	}

	// Determine filename from URL
	// Remove query parameters if any
	cleanEndpoint := strings.Split(endpoint, "?")[0]
	parts := strings.Split(cleanEndpoint, "/")
	fileName := parts[len(parts)-1]
	if fileName == "" {
		fileName = "unknown"
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".xml") {
		fileName += ".xml"
	}

	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		fmt.Printf("Failed to write XML dump to %s: %v\n", filePath, err)
	} else {
		// fmt.Printf("Saved XML dump to %s\n", filePath)
	}
}
