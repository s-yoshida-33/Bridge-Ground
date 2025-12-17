package sync

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/models"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
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
	Config             *config.Config
	DB                 *db.Manager
	progressCallback   func(SyncProgress)
	dataUpdateCallback func(string)
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

func (m *Manager) SetDataUpdateCallback(cb func(string)) {
	m.dataUpdateCallback = cb
}

func (m *Manager) notifyProgress(p SyncProgress) {
	if m.progressCallback != nil {
		m.progressCallback(p)
	}
}

func (m *Manager) notifyDataUpdate(resourceName string) {
	if m.dataUpdateCallback != nil {
		m.dataUpdateCallback(resourceName)
	}
}

func (m *Manager) getLastUpdateDateAll(key string) (string, error) {
	var val string
	// Check if table exists first? No, InitializeSchema ensures it exists.
	// But during first run after migration, it might be empty.
	err := m.DB.Conn.QueryRow("SELECT value FROM sync_meta WHERE key = ?", key).Scan(&val)
	if err != nil {
		// sql.ErrNoRows or other error
		return "", nil
	}
	return val, nil
}

func (m *Manager) setLastUpdateDateAll(key, value string) error {
	_, err := m.DB.Conn.Exec("INSERT OR REPLACE INTO sync_meta (key, value) VALUES (?, ?)", key, value)
	return err
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

		id = strings.TrimSpace(id) // Ensure ID is trimmed
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
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("shops")
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
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("shop_news")
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
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("event_news")
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
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("specials")
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
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("genres")
		}
		// Genre updates are not tracked for "updated" flag currently (always 0)
	} else {
		fmt.Println("Skipping genres sync (disabled in config)")
	}

	// 6. Cleanup Orphaned Files
	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 95, Message: "古いファイルを整理中..."},
	})
	if err := m.cleanupOrphanedFiles(); err != nil {
		fmt.Printf("Error cleaning up files: %v\n", err)
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
			// Log error
			fmt.Printf("[Worker] Failed to download %s: %v\n", job.RemoteURL, err)
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

	// Helper to perform download
	doDownload := func(targetUrl string) error {
		req, err := http.NewRequest("GET", targetUrl, nil)
		if err != nil {
			return err
		}

		if m.Config.APISettings.Username != "" {
			req.SetBasicAuth(m.Config.APISettings.Username, m.Config.APISettings.Password)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status: %s", resp.Status)
		}

		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		return err
	}

	// First attempt
	err := doDownload(url)
	if err == nil {
		fmt.Printf("[Download] Success: %s\n", url)
		return nil
	}

	// If failed and URL contains "/api/", try removing it (common path issue)
	if strings.Contains(url, "/api/") {
		altUrl := strings.Replace(url, "/api/", "/", 1)
		fmt.Printf("[Download] Retrying with alternative URL: %s (Original error: %v)\n", altUrl, err)
		if errRetry := doDownload(altUrl); errRetry == nil {
			fmt.Printf("[Download] Success on retry: %s\n", altUrl)
			return nil
		}
	}

	return fmt.Errorf("failed to download file: %s, error: %w", url, err)
}

func (m *Manager) getBaseFileDir() string {
	appData, _ := os.UserConfigDir()
	return filepath.Join(appData, "TTI", "BridgeGround", "files")
}

func (m *Manager) resolveURL(relativePath string) string {
	base := strings.TrimRight(m.Config.APISettings.BaseURL, "/")
	cleanRel := strings.TrimLeft(relativePath, "/")

	// If the relative path starts with "files/", check if base URL ends with "api".
	// If so, we likely want to strip "api" from base to get the file root.
	// Example: Base=".../api", Rel="files/..." -> ".../files/..."
	if strings.HasPrefix(cleanRel, "files/") && strings.HasSuffix(base, "/api") {
		base = strings.TrimSuffix(base, "/api")
		// Trim again in case there was a slash before "api"
		base = strings.TrimRight(base, "/")
	}

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
	endpoint := baseURL + "shoplist?limit=1000"
	fmt.Printf("Fetching shops from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.ShopListResponse

	// Create XML decoder with CharsetReader to handle non-UTF-8 encodings (like Shift_JIS)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		// 簡易的な実装: 実際には iconv ライブラリなどを使うのがベストだが、
		// Go標準ではサポートが薄いため、必要ならここに変換ロジックを入れる。
		// 現在は "utf-8" 以外が来た場合もそのまま通しているが、
		// 文字化けの原因がXMLパース時のエンコーディング不一致にあるならここで対処が必要。
		// もしサーバーが "Windows-31J" や "Shift_JIS" を返しているなら変換が必要。
		return input, nil
	}

	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		lastUpdate, _ := m.getLastUpdateDateAll("shops")
		if lastUpdate == resp.UpdateDateAll {
			fmt.Println("Shops data is up to date (UpdateDateAll match). Skipping.")
			return 0, nil
		}
	}

	existing, _ := m.getExistingUpdateDates("shops", "shop_id")
	// Make a copy or just assume existing map will be used to track deletions
	// But `existing` map is also used to check old date.
	// We will delete keys from `existing` as we process them.
	// Remaining keys will be deleted from DB.

	fmt.Printf("Initial existing shops count: %d\n", len(existing))

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
		item.ShopID = strings.TrimSpace(item.ShopID) // Trim ID to ensure match

		// Check update date
		oldDate, exists := existing[item.ShopID]
		if exists {
			delete(existing, item.ShopID) // Mark as seen
		}

		// Determine if update is needed
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

		item.ShopName = repairMojibake(item.ShopName)
		item.ShopNameKana = repairMojibake(item.ShopNameKana)
		item.ShopNameEnglish = repairMojibake(item.ShopNameEnglish)
		item.Description = repairMojibake(item.Description)
		item.Genre = repairMojibake(item.Genre)
		item.GenreSub = repairMojibake(item.GenreSub)
		item.GenreSubEnglish = repairMojibake(item.GenreSubEnglish)
		item.GenreMemo = repairMojibake(item.GenreMemo)
		item.GenreMemoEnglish = repairMojibake(item.GenreMemoEnglish)
		item.Searches = repairMojibake(item.Searches)
		item.Floors = repairMojibake(item.Floors)
		item.Area = repairMojibake(item.Area)
		item.AreaSub = repairMojibake(item.AreaSub)
		item.OpenTime = repairMojibake(item.OpenTime)

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

	// Delete obsolete records
	deletedCount := 0
	if len(existing) > 0 {
		fmt.Printf("Deleting %d obsolete shops\n", len(existing))
		delStmt, err := tx.Prepare("DELETE FROM shops WHERE shop_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existing {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
			} else {
				fmt.Printf("Failed to delete shop %s: %v\n", id, err)
			}
		}
		fmt.Printf("Deleted %d shops\n", deletedCount)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("shops", resp.UpdateDateAll)
	}

	m.processDownloads(downloadJobs)
	return updateCount - deletedCount, nil
}

func (m *Manager) syncShopNews() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "shopnewslist?limit=1000"
	fmt.Printf("Fetching shop news from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.ShopNewsResponse

	// Use decoder for consistent handling
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}
	fmt.Printf("Parsed %d items from XML for Shop News\n", len(resp.Items))

	if resp.UpdateDateAll != "" {
		lastUpdate, _ := m.getLastUpdateDateAll("shop_news")
		if lastUpdate == resp.UpdateDateAll {
			fmt.Println("Shop News data is up to date (UpdateDateAll match). Skipping.")
			return 0, nil
		}
	}

	existing, _ := m.getExistingUpdateDates("shop_news", "shop_news_id")
	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO shop_news (
		shop_news_id, shop_id, shop_name, shop_logo, shop_floors_name, 
		title, body, categories, date_start, date_end, photo1,
		photo1_remote_url, photo1_local_path, shop_logo_remote_url, shop_logo_local_path, 
		update_date
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		item.ShopNewsID = strings.TrimSpace(item.ShopNewsID)
		// Check update date
		oldDate, exists := existing[item.ShopNewsID]
		if exists {
			delete(existing, item.ShopNewsID)
		}

		if !exists || (oldDate != item.UpdateDate && item.UpdateDate != "" && strings.TrimSpace(oldDate) != strings.TrimSpace(item.UpdateDate)) {
			updateCount++
		}

		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		if item.ShopLogo != "" {
			item.ShopLogoRemoteURL = m.resolveURL(item.ShopLogo)
			item.ShopLogoLocalPath = m.resolveLocalPath(baseFileDir, item.ShopLogo)
			downloadJobs = append(downloadJobs, DownloadJob{item.ShopLogoRemoteURL, item.ShopLogoLocalPath})
		}

		item.Title = repairMojibake(item.Title)
		item.Body = repairMojibake(item.Body)
		item.Categories = repairMojibake(item.Categories)
		item.ShopName = repairMojibake(item.ShopName)
		item.ShopFloorsName = repairMojibake(item.ShopFloorsName)

		_, err := stmt.Exec(
			item.ShopNewsID, item.ShopID, item.ShopName, item.ShopLogo, item.ShopFloorsName,
			item.Title, item.Body, item.Categories, item.DateStart, item.DateEnd, item.Photo1,
			item.Photo1RemoteURL, item.Photo1LocalPath, item.ShopLogoRemoteURL, item.ShopLogoLocalPath,
			item.UpdateDate,
		)
		if err != nil {
			fmt.Printf("Error inserting shop news %s: %v\n", item.ShopNewsID, err)
		}
	}

	// Delete obsolete records
	deletedCount := 0
	if len(existing) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM shop_news WHERE shop_news_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existing {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("shop_news", resp.UpdateDateAll)
	}

	m.processDownloads(downloadJobs)
	return updateCount - deletedCount, nil
}

func (m *Manager) syncEventNews() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "eventnewslist?limit=1000"
	fmt.Printf("Fetching event news from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.EventNewsResponse

	// Use decoder for consistent handling
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}
	fmt.Printf("Parsed %d items from XML for Event News\n", len(resp.Items))

	if resp.UpdateDateAll != "" {
		lastUpdate, _ := m.getLastUpdateDateAll("event_news")
		if lastUpdate == resp.UpdateDateAll {
			fmt.Println("Event News data is up to date (UpdateDateAll match). Skipping.")
			return 0, nil
		}
	}

	existing, _ := m.getExistingUpdateDates("event_news", "event_id")
	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO event_news (
		event_id, title, body, categories, date_start, date_end, display_end, venues, photo1,
		photo1_remote_url, photo1_local_path, update_date
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		item.EventID = strings.TrimSpace(item.EventID)
		// Check update date
		oldDate, exists := existing[item.EventID]
		if exists {
			delete(existing, item.EventID)
		}

		if !exists || (oldDate != item.UpdateDate && item.UpdateDate != "" && strings.TrimSpace(oldDate) != strings.TrimSpace(item.UpdateDate)) {
			updateCount++
		}

		// Normalize Venues (remove surrounding whitespace which might happen with CDATA formatting in XML)
		item.Venues = strings.TrimSpace(item.Venues)

		item.Title = repairMojibake(item.Title)
		item.Body = repairMojibake(item.Body)
		item.Categories = repairMojibake(item.Categories)
		item.Venues = repairMojibake(item.Venues)

		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		_, err := stmt.Exec(
			item.EventID, item.Title, item.Body, item.Categories, item.DateStart, item.DateEnd,
			item.DisplayEnd, item.Venues, item.Photo1,
			item.Photo1RemoteURL, item.Photo1LocalPath, item.UpdateDate,
		)
		if err != nil {
			fmt.Printf("Error inserting event news %s: %v\n", item.EventID, err)
		}
	}

	// Delete obsolete records
	deletedCount := 0
	if len(existing) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM event_news WHERE event_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existing {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("event_news", resp.UpdateDateAll)
	}

	m.processDownloads(downloadJobs)
	return updateCount - deletedCount, nil
}

func (m *Manager) syncSpecials() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "speciallist?limit=1000"
	fmt.Printf("Fetching specials from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.SpecialListResponse

	// Use decoder for consistent handling
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		lastUpdate, _ := m.getLastUpdateDateAll("specials")
		if lastUpdate == resp.UpdateDateAll {
			fmt.Println("Specials data is up to date (UpdateDateAll match). Skipping.")
			return 0, nil
		}
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

			item.SpecialID = strings.TrimSpace(item.SpecialID)
			// Check update date
			oldDate, exists := existing[item.SpecialID]
			if exists {
				delete(existing, item.SpecialID)
			}

			if !exists || (oldDate != item.UpdateDate && item.UpdateDate != "" && strings.TrimSpace(oldDate) != strings.TrimSpace(item.UpdateDate)) {
				updateCount++
			}

			if item.SpecialImage != "" {
				item.SpecialImageRemoteURL = m.resolveURL(item.SpecialImage)
				item.SpecialImageLocalPath = m.resolveLocalPath(baseFileDir, item.SpecialImage)
				downloadJobs = append(downloadJobs, DownloadJob{item.SpecialImageRemoteURL, item.SpecialImageLocalPath})
			}

			item.SpecialTitle = repairMojibake(item.SpecialTitle)
			item.Title = repairMojibake(item.Title)
			item.SpecialSubBody = repairMojibake(item.SpecialSubBody)
			item.CategoryName = repairMojibake(item.CategoryName)
			item.ShopName = repairMojibake(item.ShopName)

			stmt.Exec(item.SpecialID, item.SpecialTitle, item.Title, item.SpecialSubBody, item.CategoryName, item.ShopID, item.ShopName, item.UpdateDate, item.SpecialImageLocalPath)
		}
	}

	// Delete obsolete records
	deletedCount := 0
	if len(existing) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM specials WHERE special_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existing {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("specials", resp.UpdateDateAll)
	}

	m.processDownloads(downloadJobs)
	return updateCount - deletedCount, nil
}

func (m *Manager) syncGenres() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "genrelist?limit=1000"
	fmt.Printf("Fetching genres from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.GenreListResponse

	// Use decoder for consistent handling
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		lastUpdate, _ := m.getLastUpdateDateAll("genres")
		if lastUpdate == resp.UpdateDateAll {
			fmt.Println("Genres data is up to date (UpdateDateAll match). Skipping.")
			return 0, nil
		}
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
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("genres", resp.UpdateDateAll)
	}

	return 0, nil
}

func (m *Manager) cleanupOrphanedFiles() error {
	baseFileDir := m.getBaseFileDir()

	// 1. Collect all valid local paths from DB
	validPaths := make(map[string]bool)

	queries := []string{
		"SELECT photo1_local_path FROM shops WHERE photo1_local_path != ''",
		"SELECT photo2_local_path FROM shops WHERE photo2_local_path != ''",
		"SELECT shop_logo_local_path FROM shops WHERE shop_logo_local_path != ''",
		"SELECT photo1_local_path FROM shop_news WHERE photo1_local_path != ''",
		"SELECT shop_logo_local_path FROM shop_news WHERE shop_logo_local_path != ''",
		"SELECT photo1_local_path FROM event_news WHERE photo1_local_path != ''",
		"SELECT special_image_local_path FROM specials WHERE special_image_local_path != ''",
	}

	for _, q := range queries {
		rows, err := m.DB.Conn.Query(q)
		if err != nil {
			fmt.Printf("Warning: cleanup query failed: %s, %v\n", q, err)
			continue
		}
		defer rows.Close() // In loop but rows closed after scan or next iter. better to use func closure if concerned about fd limit
		// Actually rows.Close() is defered but will stack up. Let's wrap in closure or just direct Close.
		// Re-writing loop body:
		func() {
			defer rows.Close()
			for rows.Next() {
				var p string
				if err := rows.Scan(&p); err == nil && p != "" {
					// Normalize path
					abs, err := filepath.Abs(p)
					if err == nil {
						validPaths[strings.ToLower(abs)] = true
					}
				}
			}
		}()
	}

	// 2. Walk through the file directory
	err := filepath.Walk(baseFileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if file is in validPaths
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil
		}

		if !validPaths[strings.ToLower(absPath)] {
			// Found orphaned file
			// fmt.Printf("Removing orphaned file: %s\n", path)
			if err := os.Remove(path); err != nil {
				fmt.Printf("Failed to remove orphaned file: %s, %v\n", path, err)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 3. Remove empty directories
	// We walk again (or could have collected dirs in pass 2)
	// Simple approach: walk, collect dirs, reverse sort by length (deepest first), remove if empty
	var dirs []string
	filepath.Walk(baseFileDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && path != baseFileDir {
			dirs = append(dirs, path)
		}
		return nil
	})

	// Sort by length descending to process deepest directories first
	// Bubble sort or simple swap for simplicity since depth isn't huge
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if len(dirs[i]) < len(dirs[j]) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	for _, dir := range dirs {
		// Try to remove. It will fail if not empty, which is what we want.
		// os.Remove on a directory only works if empty.
		if err := os.Remove(dir); err == nil {
			// fmt.Printf("Removed empty directory: %s\n", dir)
		}
	}

	return nil
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

	// 修正: レスポンスの文字コードを考慮して読み込む
	// XMLは通常ヘッダーでエンコーディングを指定するが、Goのxml.Unmarshalは
	// デフォルトでUTF-8しかサポートしておらず、Shift-JIS等が来ると化ける可能性がある。
	// charset readerを設定する必要があるが、ここでは簡易的にデータをすべて読み込んでから
	// 必要なら変換を行うアプローチをとるか、あるいはxml.DecoderにCharsetReaderを設定する。

	// ここではまず生データを返す。呼び出し元（syncShops等）のxml.Unmarshalで
	// 適切なCharsetReaderを設定することで対応する。

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

	// Try to repair mojibake before saving for better readability
	content := string(data)
	repaired := repairMojibake(content)

	if err := os.WriteFile(filePath, []byte(repaired), 0644); err != nil {
		fmt.Printf("Failed to write XML dump to %s: %v\n", filePath, err)
	} else {
		// fmt.Printf("Saved XML dump to %s\n", filePath)
	}
}

// repairMojibake attempts to fix garbled text caused by UTF-8 bytes being interpreted as Windows-1252.
// e.g. "ã€" (E3 80 90 interpreted as Win1252) -> "【" (E3 80 90 as UTF-8)
func repairMojibake(s string) string {
	// Windows-1252 specific mappings for 0x80-0x9F range
	win1252 := map[rune]byte{
		0x20AC: 0x80, 0x201A: 0x82, 0x0192: 0x83, 0x201E: 0x84,
		0x2026: 0x85, 0x2020: 0x86, 0x2021: 0x87, 0x02C6: 0x88,
		0x2030: 0x89, 0x0160: 0x8A, 0x2039: 0x8B, 0x0152: 0x8C,
		0x017D: 0x8E, 0x2018: 0x91, 0x2019: 0x92, 0x201C: 0x93,
		0x201D: 0x94, 0x2022: 0x95, 0x2013: 0x96, 0x2014: 0x97,
		0x02DC: 0x98, 0x2122: 0x99, 0x0161: 0x9A, 0x203A: 0x9B,
		0x0153: 0x9C, 0x017E: 0x9E, 0x0178: 0x9F,
	}

	var buf bytes.Buffer
	// Pre-allocate buffer estimation
	buf.Grow(len(s))

	for _, r := range s {
		if r <= 0x7F {
			// ASCII is safe
			buf.WriteByte(byte(r))
		} else if b, ok := win1252[r]; ok {
			// Map back to specific byte
			buf.WriteByte(b)
		} else if r >= 0xA0 && r <= 0xFF {
			// Identity mapping for Latin-1 Supplement
			buf.WriteByte(byte(r))
		} else if r >= 0x80 && r <= 0x9F {
			// Pass through C1 controls (sometimes mapped directly if not in win1252 map)
			buf.WriteByte(byte(r))
		} else {
			// If we encounter a character that couldn't have come from CP1252 decoding
			// (e.g. Japanese character), then this string is likely NOT mojibake (or at least not this type).
			// Return original string to be safe.
			return s
		}
	}

	res := buf.Bytes()
	// Check if the result is valid UTF-8
	if utf8.Valid(res) {
		return string(res)
	}
	// If not valid UTF-8, return original
	return s
}
