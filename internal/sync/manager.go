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
	"time"
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

type DownloadResult struct {
	LocalPath string
	Success   bool
	Error     error
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


func (m *Manager) setLastUpdateDateAll(key, value string) error {
	_, err := m.DB.Conn.Exec("INSERT OR REPLACE INTO sync_meta (key, value) VALUES (?, ?)", key, value)
	return err
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
			Floors:    true,
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
			Main: &ProgressDetail{Percentage: 85, Message: "ジャンルデータを同期中..."},
		})
		count, err := m.syncGenres()
		if err != nil {
			fmt.Printf("Error syncing genres: %v\n", err)
		}
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("genres")
		}
		updateStats["genres"] = count
	} else {
		fmt.Println("Skipping genres sync (disabled in config)")
	}

	// 6. Sync Floors
	if targets.Floors {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 90, Message: "フロアデータを同期中..."},
		})
		count, err := m.syncFloors()
		if err != nil {
			fmt.Printf("Error syncing floors: %v\n", err)
		}
		if count != 0 {
			anyUpdated = true
			m.notifyDataUpdate("floors")
		}
		updateStats["floors"] = count
	} else {
		fmt.Println("Skipping floors sync (disabled in config)")
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

// shouldDownloadImage determines if image file should be downloaded
// Returns true if: file doesn't exist OR file is older than API update date
func (m *Manager) shouldDownloadImage(destPath string, updateDateStr string, oldImagePath string) bool {
	// Always download if path changed
	if destPath != oldImagePath {
		fmt.Printf("[Download] Path changed: %q -> %q, will download\n", oldImagePath, destPath)
		return true
	}

	// Check if file exists
	fileInfo, err := os.Stat(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[Download] File doesn't exist: %q, will download\n", destPath)
			return true
		}
		// Unknown error - download to be safe
		fmt.Printf("[Download] Stat error: %v, downloading to be safe\n", err)
		return true
	}

	// File exists - compare timestamps
	fileMTime := fileInfo.ModTime()

	// Parse API update date string (format: "2024-01-15 10:30:00")
	apiUpdateTime, err := time.Parse("2006-01-02 15:04:05", updateDateStr)
	if err != nil {
		fmt.Printf("[Download] Failed to parse update date %q: %v, downloading to be safe\n", updateDateStr, err)
		return true
	}

	// If API is newer, download
	if apiUpdateTime.After(fileMTime) {
		fmt.Printf("[Download] API newer than file: API=%v, File=%v, will download %q\n", apiUpdateTime, fileMTime, destPath)
		return true
	}

	fmt.Printf("[Download] File is up-to-date (API=%v, File=%v), skipping %q\n", apiUpdateTime, fileMTime, destPath)
	return false
}

// downloadWorker processes download jobs from the channel
func (m *Manager) downloadWorker(jobs <-chan DownloadJob, results chan<- DownloadResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		err := m.downloadFile(job.RemoteURL, job.LocalPath)
		results <- DownloadResult{
			LocalPath: job.LocalPath,
			Success:   err == nil,
			Error:     err,
		}
		if err != nil {
			// Log error
			fmt.Printf("[Worker] Failed to download %s: %v\n", job.RemoteURL, err)
		}
	}
}

func (m *Manager) downloadFile(url, destPath string) error {
	if url == "" || destPath == "" {
		fmt.Printf("[Download] Skipped (empty URL or dest): URL=%q, Dest=%q\n", url, destPath)
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

func (m *Manager) processDownloads(jobs []DownloadJob) int {
	fmt.Printf("=== Processing %d download jobs ===\n", len(jobs))
	for i, job := range jobs {
		fmt.Printf("  [%d] URL: %s\n", i+1, job.RemoteURL)
		fmt.Printf("       To: %s\n", job.LocalPath)
	}
	
	jobChan := make(chan DownloadJob, len(jobs))
	resultChan := make(chan DownloadResult, len(jobs))
	var wg sync.WaitGroup

	workerCount := 5
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go m.downloadWorker(jobChan, resultChan, &wg)
	}

	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)
	wg.Wait()
	close(resultChan)
	
	// Count successful downloads
	successCount := 0
	for result := range resultChan {
		if result.Success {
			successCount++
			// Verify file exists after download
			if _, err := os.Stat(result.LocalPath); err != nil {
				fmt.Printf("[Verify] WARNING: File not found after successful download: %q\n", result.LocalPath)
			} else {
				fmt.Printf("[Verify] OK: File exists after download: %q\n", result.LocalPath)
			}
		} else if result.Error != nil {
			fmt.Printf("[Download] FAILED: %s (error: %v)\n", result.LocalPath, result.Error)
		}
	}
	
	fmt.Printf("=== Download processing completed: %d successful ===\n", successCount)
	return successCount
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
		return input, nil
	}

	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}
	
	fmt.Printf("Parsed %d shops from XML\n", len(resp.Items))
	if len(resp.Items) > 0 {
		firstItem := resp.Items[0]
		fmt.Printf("[First shop #%s] Thumbnails:\n", firstItem.ShopID)
		fmt.Printf("  Photo1ThumbW640: %q\n", firstItem.Photo1ThumbW640)
		fmt.Printf("  Photo2ThumbW640: %q\n", firstItem.Photo2ThumbW640)
		fmt.Printf("  ShopLogoThumb640x640: %q\n", firstItem.ShopLogoThumb640x640)
		fmt.Printf("  ShopLogoThumbW640: %q\n", firstItem.ShopLogoThumbW640)
	}
	existingShops, err := m.loadAllShops()
	if err != nil {
		// If table doesn't exist, it might be fine on first run, but loadAllShops handles that via empty map if needed?
		// Actually loadAllShops returns error if query fails. But syncShops is called after InitializeSchema so it should be fine.
		// If query fails for other reasons, we might want to proceed as empty.
		fmt.Printf("Warning: Failed to load existing shops: %v. Assuming empty.\n", err)
		existingShops = make(map[string]models.ShopItem)
	}

	fmt.Printf("Initial existing shops count: %d\n", len(existingShops))

	updateCount := 0
	
	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO shops (
		shop_id, shop_name, shop_name_kana, shop_name_english,
		shop_name_china_cn, shop_name_china_tw, shop_name_korea, shop_name_france, shop_name_vietnam, shop_name_thai,
		abbr, web_status, searches,
		genre, genre_sub, genre_sub_english,
		genre_memo, genre_memo_english, genre_memo_china_cn, genre_memo_china_tw, genre_memo_korea, genre_memo_france, genre_memo_vietnam, genre_memo_thai,
		group_id, tenant_code, tel, user_url,
		floor, floors, area, area_sub, number,
		open_year, open_month, open_day, close_flg, pub_start, pub_end, open_time, description,
		qr, food_class, seats, smoking, reservation, lunch_menu, dinner_menu, take_out, childrens_menu, baby_seat, alcohol, options,
		photo1, photo1_remote_url, photo1_local_path,
		photo2, photo2_remote_url, photo2_local_path,
		shop_logo, shop_logo_remote_url, shop_logo_local_path,
		photo1_thumb, photo1_thumb_150x150, photo1_thumb_640x640, photo1_thumb_w320,
		photo1_thumb_w640, photo1_thumb_w640_remote_url, photo1_thumb_w640_local_path,
		photo2_thumb, photo2_thumb_150x150, photo2_thumb_640x640, photo2_thumb_w320,
		photo2_thumb_w640, photo2_thumb_w640_remote_url, photo2_thumb_w640_local_path,
		shop_logo_thumb, shop_logo_thumb_150x150,
		shop_logo_thumb_640x640, shop_logo_thumb_640x640_remote_url, shop_logo_thumb_640x640_local_path,
		shop_logo_thumb_w320,
		shop_logo_thumb_w640, shop_logo_thumb_w640_remote_url, shop_logo_thumb_w640_local_path,
		update_date
	) VALUES (
		?, ?, ?, ?,
		?, ?, ?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?,
		?, ?,
		?, ?, ?,
		?,
		?, ?, ?,
		?
	)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		item.ShopID = strings.TrimSpace(item.ShopID)
		
		// Repair Mojibake first
		item.ShopName = repairMojibake(item.ShopName)
		item.ShopNameKana = repairMojibake(item.ShopNameKana)
		item.ShopNameEnglish = repairMojibake(item.ShopNameEnglish)
		item.ShopNameChinaCN = repairMojibake(item.ShopNameChinaCN)
		item.ShopNameChinaTW = repairMojibake(item.ShopNameChinaTW)
		item.ShopNameKorea = repairMojibake(item.ShopNameKorea)
		item.ShopNameFrance = repairMojibake(item.ShopNameFrance)
		item.ShopNameVietnam = repairMojibake(item.ShopNameVietnam)
		item.ShopNameThai = repairMojibake(item.ShopNameThai)
		item.Description = repairMojibake(item.Description)
		item.Genre = repairMojibake(item.Genre)
		item.GenreSub = repairMojibake(item.GenreSub)
		item.GenreSubEnglish = repairMojibake(item.GenreSubEnglish)
		item.GenreMemo = repairMojibake(item.GenreMemo)
		item.GenreMemoEnglish = repairMojibake(item.GenreMemoEnglish)
		item.GenreMemoChinaCN = repairMojibake(item.GenreMemoChinaCN)
		item.GenreMemoChinaTW = repairMojibake(item.GenreMemoChinaTW)
		item.GenreMemoKorea = repairMojibake(item.GenreMemoKorea)
		item.GenreMemoFrance = repairMojibake(item.GenreMemoFrance)
		item.GenreMemoVietnam = repairMojibake(item.GenreMemoVietnam)
		item.GenreMemoThai = repairMojibake(item.GenreMemoThai)
		item.Searches = repairMojibake(item.Searches)
		item.Floors = repairMojibake(item.Floors)
		item.Area = repairMojibake(item.Area)
		item.AreaSub = repairMojibake(item.AreaSub)
		item.OpenTime = repairMojibake(item.OpenTime)

		// Check against existing
		existingItem, exists := existingShops[item.ShopID]
		needsUpdate := true
		
		if exists {
			// Compare fields
			if shopsEqual(item, existingItem) {
				needsUpdate = false
			}
			// Mark as processed by removing from map
			delete(existingShops, item.ShopID)
		}

		if needsUpdate {
			updateCount++
			
			fmt.Printf("[Shop %s] Processing images...\n", item.ShopID)
			
			// Handle images: download if path changed or file doesn't exist
			if item.Photo1 != "" {
				item.Photo1RemoteURL = m.resolveURL(item.Photo1)
				item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
				fmt.Printf("  [Photo1] Remote=%q, Local=%q\n", item.Photo1RemoteURL, item.Photo1LocalPath)
				if m.shouldDownloadImage(item.Photo1LocalPath, item.UpdateDate, existingItem.Photo1) {
					fmt.Printf("  [Photo1] Adding to download queue\n")
					downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
				}
			}
			if item.Photo2 != "" {
				item.Photo2RemoteURL = m.resolveURL(item.Photo2)
				item.Photo2LocalPath = m.resolveLocalPath(baseFileDir, item.Photo2)
				if m.shouldDownloadImage(item.Photo2LocalPath, item.UpdateDate, existingItem.Photo2) {
					downloadJobs = append(downloadJobs, DownloadJob{item.Photo2RemoteURL, item.Photo2LocalPath})
				}
			}
			if item.ShopLogo != "" {
				item.ShopLogoRemoteURL = m.resolveURL(item.ShopLogo)
				item.ShopLogoLocalPath = m.resolveLocalPath(baseFileDir, item.ShopLogo)
				if m.shouldDownloadImage(item.ShopLogoLocalPath, item.UpdateDate, existingItem.ShopLogo) {
					downloadJobs = append(downloadJobs, DownloadJob{item.ShopLogoRemoteURL, item.ShopLogoLocalPath})
				}
			}
			if item.Photo1ThumbW640 != "" {
				item.Photo1ThumbW640RemoteURL = m.resolveURL(item.Photo1ThumbW640)
				item.Photo1ThumbW640LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1ThumbW640)
				fmt.Printf("  [Photo1ThumbW640] Remote=%q, Local=%q\n", item.Photo1ThumbW640RemoteURL, item.Photo1ThumbW640LocalPath)
				if m.shouldDownloadImage(item.Photo1ThumbW640LocalPath, item.UpdateDate, existingItem.Photo1ThumbW640) {
					fmt.Printf("  [Photo1ThumbW640] Adding to download queue\n")
					downloadJobs = append(downloadJobs, DownloadJob{item.Photo1ThumbW640RemoteURL, item.Photo1ThumbW640LocalPath})
				} else {
					fmt.Printf("  [Photo1ThumbW640] Skip download\n")
				}
			} else {
				fmt.Printf("  [Photo1ThumbW640] Empty (no thumbnail)\n")
			}
			if item.Photo2ThumbW640 != "" {
				item.Photo2ThumbW640RemoteURL = m.resolveURL(item.Photo2ThumbW640)
				item.Photo2ThumbW640LocalPath = m.resolveLocalPath(baseFileDir, item.Photo2ThumbW640)
				fmt.Printf("  [Photo2ThumbW640] Remote=%q, Local=%q\n", item.Photo2ThumbW640RemoteURL, item.Photo2ThumbW640LocalPath)
				if m.shouldDownloadImage(item.Photo2ThumbW640LocalPath, item.UpdateDate, existingItem.Photo2ThumbW640) {
					fmt.Printf("  [Photo2ThumbW640] Adding to download queue\n")
					downloadJobs = append(downloadJobs, DownloadJob{item.Photo2ThumbW640RemoteURL, item.Photo2ThumbW640LocalPath})
				} else {
					fmt.Printf("  [Photo2ThumbW640] Skip download\n")
				}
			} else {
				fmt.Printf("  [Photo2ThumbW640] Empty (no thumbnail)\n")
			}
			if item.ShopLogoThumb640x640 != "" {
				item.ShopLogoThumb640x640RemoteURL = m.resolveURL(item.ShopLogoThumb640x640)
				item.ShopLogoThumb640x640LocalPath = m.resolveLocalPath(baseFileDir, item.ShopLogoThumb640x640)
				fmt.Printf("  [ShopLogoThumb640x640] Remote=%q, Local=%q\n", item.ShopLogoThumb640x640RemoteURL, item.ShopLogoThumb640x640LocalPath)
				if m.shouldDownloadImage(item.ShopLogoThumb640x640LocalPath, item.UpdateDate, existingItem.ShopLogoThumb640x640) {
					fmt.Printf("  [ShopLogoThumb640x640] Adding to download queue\n")
					downloadJobs = append(downloadJobs, DownloadJob{item.ShopLogoThumb640x640RemoteURL, item.ShopLogoThumb640x640LocalPath})
				} else {
					fmt.Printf("  [ShopLogoThumb640x640] Skip download\n")
				}
			} else {
				fmt.Printf("  [ShopLogoThumb640x640] Empty (no thumbnail)\n")
			}
			if item.ShopLogoThumbW640 != "" {
				item.ShopLogoThumbW640RemoteURL = m.resolveURL(item.ShopLogoThumbW640)
				item.ShopLogoThumbW640LocalPath = m.resolveLocalPath(baseFileDir, item.ShopLogoThumbW640)
				fmt.Printf("  [ShopLogoThumbW640] Remote=%q, Local=%q\n", item.ShopLogoThumbW640RemoteURL, item.ShopLogoThumbW640LocalPath)
				if m.shouldDownloadImage(item.ShopLogoThumbW640LocalPath, item.UpdateDate, existingItem.ShopLogoThumbW640) {
					fmt.Printf("  [ShopLogoThumbW640] Adding to download queue\n")
					downloadJobs = append(downloadJobs, DownloadJob{item.ShopLogoThumbW640RemoteURL, item.ShopLogoThumbW640LocalPath})
				} else {
					fmt.Printf("  [ShopLogoThumbW640] Skip download\n")
				}
			} else {
				fmt.Printf("  [ShopLogoThumbW640] Empty (no thumbnail)\n")
			}

			if _, err := stmt.Exec(
				item.ShopID, item.ShopName, item.ShopNameKana, item.ShopNameEnglish,
				item.ShopNameChinaCN, item.ShopNameChinaTW, item.ShopNameKorea, item.ShopNameFrance, item.ShopNameVietnam, item.ShopNameThai,
				item.Abbr, item.WebStatus, item.Searches,
				item.Genre, item.GenreSub, item.GenreSubEnglish,
				item.GenreMemo, item.GenreMemoEnglish, item.GenreMemoChinaCN, item.GenreMemoChinaTW, item.GenreMemoKorea, item.GenreMemoFrance, item.GenreMemoVietnam, item.GenreMemoThai,
				item.GroupID, item.TenantCode, item.Tel, item.UserUrl,
				item.Floor, item.Floors, item.Area, item.AreaSub, item.Number,
				item.OpenYear, item.OpenMonth, item.OpenDay, item.CloseFlg, item.PubStart, item.PubEnd, item.OpenTime, item.Description,
				item.Qr, item.FoodClass, item.Seats, item.Smoking, item.Reservation, item.LunchMenu, item.DinnerMenu, item.TakeOut, item.ChildrensMenu, item.BabySeat, item.Alcohol, item.Options,
				item.Photo1, item.Photo1RemoteURL, item.Photo1LocalPath,
				item.Photo2, item.Photo2RemoteURL, item.Photo2LocalPath,
				item.ShopLogo, item.ShopLogoRemoteURL, item.ShopLogoLocalPath,
				item.Photo1Thumb, item.Photo1Thumb150x150, item.Photo1Thumb640x640, item.Photo1ThumbW320,
				item.Photo1ThumbW640, item.Photo1ThumbW640RemoteURL, item.Photo1ThumbW640LocalPath,
				item.Photo2Thumb, item.Photo2Thumb150x150, item.Photo2Thumb640x640, item.Photo2ThumbW320,
				item.Photo2ThumbW640, item.Photo2ThumbW640RemoteURL, item.Photo2ThumbW640LocalPath,
				item.ShopLogoThumb, item.ShopLogoThumb150x150,
				item.ShopLogoThumb640x640, item.ShopLogoThumb640x640RemoteURL, item.ShopLogoThumb640x640LocalPath,
				item.ShopLogoThumbW320,
				item.ShopLogoThumbW640, item.ShopLogoThumbW640RemoteURL, item.ShopLogoThumbW640LocalPath,
				item.UpdateDate,
			); err != nil {
				tx.Rollback()
				fmt.Printf("ERROR: Failed to insert shop %s: %v\n", item.ShopID, err)
				return 0, fmt.Errorf("insert shop failed: %w", err)
			}
		}
	}

	// Delete obsolete records (remaining in existingShops)
	deletedCount := 0
	if len(existingShops) > 0 {
		fmt.Printf("Deleting %d obsolete shops\n", len(existingShops))
		delStmt, err := tx.Prepare("DELETE FROM shops WHERE shop_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existingShops {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
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

	fmt.Printf("=== syncShops: About to process downloads: %d jobs total ===\n", len(downloadJobs))
	downloadedCount := m.processDownloads(downloadJobs)
	
	// Return total changes: updates + deletes + successful downloads
	// If images were downloaded successfully, we consider that a data update for SSE notification
	totalChanges := updateCount + deletedCount + downloadedCount
	fmt.Printf("=== syncShops result: updateCount=%d, deletedCount=%d, downloadedCount=%d, total=%d ===\n", 
		updateCount, deletedCount, downloadedCount, totalChanges)
	return totalChanges, nil
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
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}
	fmt.Printf("Parsed %d items from XML for Shop News\n", len(resp.Items))

	existingNews, err := m.loadAllShopNews()
	if err != nil {
		fmt.Printf("Warning: Failed to load existing shop news: %v. Assuming empty.\n", err)
		existingNews = make(map[string]models.ShopNewsItem)
	}

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

		item.Title = repairMojibake(item.Title)
		item.Body = repairMojibake(item.Body)
		item.Categories = repairMojibake(item.Categories)
		item.ShopName = repairMojibake(item.ShopName)
		item.ShopFloorsName = repairMojibake(item.ShopFloorsName)

		existingItem, exists := existingNews[item.ShopNewsID]
		needsUpdate := true
		if exists {
			if shopNewsEqual(item, existingItem) {
				needsUpdate = false
			}
			delete(existingNews, item.ShopNewsID)
		}

		if needsUpdate {
			updateCount++

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

			if _, err := stmt.Exec(
				item.ShopNewsID, item.ShopID, item.ShopName, item.ShopLogo, item.ShopFloorsName,
				item.Title, item.Body, item.Categories, item.DateStart, item.DateEnd, item.Photo1,
				item.Photo1RemoteURL, item.Photo1LocalPath, item.ShopLogoRemoteURL, item.ShopLogoLocalPath,
				item.UpdateDate,
			); err != nil {
				tx.Rollback()
				fmt.Printf("ERROR: Failed to insert shop news %s: %v\n", item.ShopNewsID, err)
				return 0, fmt.Errorf("insert shop news failed: %w", err)
			}
		}
	}

	deletedCount := 0
	if len(existingNews) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM shop_news WHERE shop_news_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existingNews {
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
	return updateCount + deletedCount, nil
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
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}
	fmt.Printf("Parsed %d items from XML for Event News\n", len(resp.Items))

	existingEvents, err := m.loadAllEventNews()
	if err != nil {
		fmt.Printf("Warning: Failed to load existing event news: %v. Assuming empty.\n", err)
		existingEvents = make(map[string]models.EventNewsItem)
	}

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

		item.Venues = strings.TrimSpace(item.Venues)
		item.Title = repairMojibake(item.Title)
		item.Body = repairMojibake(item.Body)
		item.Categories = repairMojibake(item.Categories)
		item.Venues = repairMojibake(item.Venues)

		existingItem, exists := existingEvents[item.EventID]
		needsUpdate := true
		if exists {
			if eventNewsEqual(item, existingItem) {
				needsUpdate = false
			}
			delete(existingEvents, item.EventID)
		}

		if needsUpdate {
			updateCount++

			if item.Photo1 != "" {
				item.Photo1RemoteURL = m.resolveURL(item.Photo1)
				item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
				downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
			}
			if _, err := stmt.Exec(
				item.EventID, item.Title, item.Body, item.Categories, item.DateStart, item.DateEnd,
				item.DisplayEnd, item.Venues, item.Photo1,
				item.Photo1RemoteURL, item.Photo1LocalPath, item.UpdateDate,
			); err != nil {
				tx.Rollback()
				fmt.Printf("ERROR: Failed to insert event %s: %v\n", item.EventID, err)
				return 0, fmt.Errorf("insert event failed: %w", err)
			}
		}
	}

	deletedCount := 0
	if len(existingEvents) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM event_news WHERE event_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existingEvents {
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
	return updateCount + deletedCount, nil
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
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}

	existingSpecials, err := m.loadAllSpecials()
	if err != nil {
		fmt.Printf("Warning: Failed to load existing specials: %v. Assuming empty.\n", err)
		existingSpecials = make(map[string]models.SpecialItem)
	}

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

	for _, parent := range resp.Items {
		for _, item := range parent.Items {
			if item.Type != "special" {
				continue
			}

			item.SpecialTitle = parent.SpecialTitle
			item.UpdateDate = parent.UpdateDate
			item.SpecialID = strings.TrimSpace(item.SpecialID)

			item.SpecialTitle = repairMojibake(item.SpecialTitle)
			item.Title = repairMojibake(item.Title)
			item.SpecialSubBody = repairMojibake(item.SpecialSubBody)
			item.CategoryName = repairMojibake(item.CategoryName)
			item.ShopName = repairMojibake(item.ShopName)

			existingItem, exists := existingSpecials[item.SpecialID]
			needsUpdate := true
			if exists {
				if specialsEqual(item, existingItem) {
					needsUpdate = false
				}
				delete(existingSpecials, item.SpecialID)
			}

			if needsUpdate {
				updateCount++

				if item.SpecialImage != "" {
					item.SpecialImageRemoteURL = m.resolveURL(item.SpecialImage)
					item.SpecialImageLocalPath = m.resolveLocalPath(baseFileDir, item.SpecialImage)
					downloadJobs = append(downloadJobs, DownloadJob{item.SpecialImageRemoteURL, item.SpecialImageLocalPath})
				}

				if _, err := stmt.Exec(item.SpecialID, item.SpecialTitle, item.Title, item.SpecialSubBody, item.CategoryName, item.ShopID, item.ShopName, item.UpdateDate, item.SpecialImageLocalPath); err != nil {
					tx.Rollback()
					fmt.Printf("ERROR: Failed to insert special %s: %v\n", item.SpecialID, err)
					return 0, fmt.Errorf("insert special failed: %w", err)
				}
			}
		}
	}

	deletedCount := 0
	if len(existingSpecials) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM specials WHERE special_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existingSpecials {
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
	return updateCount + deletedCount, nil
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
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}

	existingGenres, err := m.loadAllGenres()
	if err != nil {
		fmt.Printf("Warning: Failed to load existing genres: %v. Assuming empty.\n", err)
		existingGenres = make(map[string]models.GenreItem)
	}

	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO genres (genre_id, genre_name, genre_slug) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, item := range resp.Items {
		item.GenreID = strings.TrimSpace(item.GenreID)

		existingItem, exists := existingGenres[item.GenreID]
		needsUpdate := true
		if exists {
			if genresEqual(item, existingItem) {
				needsUpdate = false
			}
			delete(existingGenres, item.GenreID)
		}

		if needsUpdate {
			updateCount++
			if _, err := stmt.Exec(item.GenreID, item.GenreName, item.GenreSlug); err != nil {
				tx.Rollback()
				fmt.Printf("ERROR: Failed to insert genre %s: %v\n", item.GenreID, err)
				return 0, fmt.Errorf("insert genre failed: %w", err)
			}
		}
	}

	deletedCount := 0
	if len(existingGenres) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM genres WHERE genre_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existingGenres {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("genres", resp.UpdateDateAll)
	}

	return updateCount + deletedCount, nil
}

func (m *Manager) cleanupOrphanedFiles() error {
	baseFileDir := m.getBaseFileDir()

	// 1. Collect all valid local paths from DB
	validPaths := make(map[string]bool)

	queries := []string{
		"SELECT photo1_local_path FROM shops WHERE photo1_local_path != ''",
		"SELECT photo2_local_path FROM shops WHERE photo2_local_path != ''",
		"SELECT shop_logo_local_path FROM shops WHERE shop_logo_local_path != ''",
		"SELECT photo1_thumb_w640_local_path FROM shops WHERE photo1_thumb_w640_local_path != ''",
		"SELECT photo2_thumb_w640_local_path FROM shops WHERE photo2_thumb_w640_local_path != ''",
		"SELECT shop_logo_thumb_640x640_local_path FROM shops WHERE shop_logo_thumb_640x640_local_path != ''",
		"SELECT shop_logo_thumb_w640_local_path FROM shops WHERE shop_logo_thumb_w640_local_path != ''",
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

// --- Helper Functions for Robust Sync ---

func (m *Manager) loadAllShops() (map[string]models.ShopItem, error) {
	query := `SELECT
		shop_id, shop_name, shop_name_kana, shop_name_english,
		COALESCE(shop_name_china_cn,''), COALESCE(shop_name_china_tw,''), COALESCE(shop_name_korea,''), COALESCE(shop_name_france,''), COALESCE(shop_name_vietnam,''), COALESCE(shop_name_thai,''),
		COALESCE(abbr,''), COALESCE(web_status,''), searches,
		genre, genre_sub, genre_sub_english,
		genre_memo, genre_memo_english,
		COALESCE(genre_memo_china_cn,''), COALESCE(genre_memo_china_tw,''), COALESCE(genre_memo_korea,''), COALESCE(genre_memo_france,''), COALESCE(genre_memo_vietnam,''), COALESCE(genre_memo_thai,''),
		group_id, COALESCE(tenant_code,''), tel, COALESCE(user_url,''),
		COALESCE(floor,''), floors, area, area_sub, number,
		COALESCE(open_year,''), COALESCE(open_month,''), COALESCE(open_day,''), close_flg, COALESCE(pub_start,''), COALESCE(pub_end,''), open_time, description,
		COALESCE(qr,''), COALESCE(food_class,''), COALESCE(seats,''), COALESCE(smoking,''), COALESCE(reservation,''),
		COALESCE(lunch_menu,''), COALESCE(dinner_menu,''), COALESCE(take_out,''), COALESCE(childrens_menu,''), COALESCE(baby_seat,''), COALESCE(alcohol,''), COALESCE(options,''),
		photo1, photo2, shop_logo, update_date,
		photo1_thumb_w640, photo2_thumb_w640, shop_logo_thumb_640x640, shop_logo_thumb_w640
		FROM shops`

	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.ShopItem)
	for rows.Next() {
		var item models.ShopItem
		var (
			shopId, shopName, shopNameKana, shopNameEnglish,
			shopNameChinaCN, shopNameChinaTW, shopNameKorea, shopNameFrance, shopNameVietnam, shopNameThai,
			abbr, webStatus, searches,
			genre, genreSub, genreSubEnglish,
			genreMemo, genreMemoEnglish,
			genreMemoChinaCN, genreMemoChinaTW, genreMemoKorea, genreMemoFrance, genreMemoVietnam, genreMemoThai,
			groupId, tenantCode, tel, userUrl,
			floor, floors, area, areaSub, number,
			openYear, openMonth, openDay, closeFlg, pubStart, pubEnd, openTime, description,
			qr, foodClass, seats, smoking, reservation,
			lunchMenu, dinnerMenu, takeOut, childrensMenu, babySeat, alcohol, options,
			photo1, photo2, shopLogo, updateDate,
			photo1ThumbW640, photo2ThumbW640, shopLogoThumb640x640, shopLogoThumbW640 *string
		)

		if err := rows.Scan(
			&shopId, &shopName, &shopNameKana, &shopNameEnglish,
			&shopNameChinaCN, &shopNameChinaTW, &shopNameKorea, &shopNameFrance, &shopNameVietnam, &shopNameThai,
			&abbr, &webStatus, &searches,
			&genre, &genreSub, &genreSubEnglish,
			&genreMemo, &genreMemoEnglish,
			&genreMemoChinaCN, &genreMemoChinaTW, &genreMemoKorea, &genreMemoFrance, &genreMemoVietnam, &genreMemoThai,
			&groupId, &tenantCode, &tel, &userUrl,
			&floor, &floors, &area, &areaSub, &number,
			&openYear, &openMonth, &openDay, &closeFlg, &pubStart, &pubEnd, &openTime, &description,
			&qr, &foodClass, &seats, &smoking, &reservation,
			&lunchMenu, &dinnerMenu, &takeOut, &childrensMenu, &babySeat, &alcohol, &options,
			&photo1, &photo2, &shopLogo, &updateDate,
			&photo1ThumbW640, &photo2ThumbW640, &shopLogoThumb640x640, &shopLogoThumbW640,
		); err != nil {
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
		item.ShopNameChinaCN = s(shopNameChinaCN)
		item.ShopNameChinaTW = s(shopNameChinaTW)
		item.ShopNameKorea = s(shopNameKorea)
		item.ShopNameFrance = s(shopNameFrance)
		item.ShopNameVietnam = s(shopNameVietnam)
		item.ShopNameThai = s(shopNameThai)
		item.Abbr = s(abbr)
		item.WebStatus = s(webStatus)
		item.Searches = s(searches)
		item.Genre = s(genre)
		item.GenreSub = s(genreSub)
		item.GenreSubEnglish = s(genreSubEnglish)
		item.GenreMemo = s(genreMemo)
		item.GenreMemoEnglish = s(genreMemoEnglish)
		item.GenreMemoChinaCN = s(genreMemoChinaCN)
		item.GenreMemoChinaTW = s(genreMemoChinaTW)
		item.GenreMemoKorea = s(genreMemoKorea)
		item.GenreMemoFrance = s(genreMemoFrance)
		item.GenreMemoVietnam = s(genreMemoVietnam)
		item.GenreMemoThai = s(genreMemoThai)
		item.GroupID = s(groupId)
		item.TenantCode = s(tenantCode)
		item.Tel = s(tel)
		item.UserUrl = s(userUrl)
		item.Floor = s(floor)
		item.Floors = s(floors)
		item.Area = s(area)
		item.AreaSub = s(areaSub)
		item.Number = s(number)
		item.OpenYear = s(openYear)
		item.OpenMonth = s(openMonth)
		item.OpenDay = s(openDay)
		item.CloseFlg = s(closeFlg)
		item.PubStart = s(pubStart)
		item.PubEnd = s(pubEnd)
		item.OpenTime = s(openTime)
		item.Description = s(description)
		item.Qr = s(qr)
		item.FoodClass = s(foodClass)
		item.Seats = s(seats)
		item.Smoking = s(smoking)
		item.Reservation = s(reservation)
		item.LunchMenu = s(lunchMenu)
		item.DinnerMenu = s(dinnerMenu)
		item.TakeOut = s(takeOut)
		item.ChildrensMenu = s(childrensMenu)
		item.BabySeat = s(babySeat)
		item.Alcohol = s(alcohol)
		item.Options = s(options)
		item.Photo1 = s(photo1)
		item.Photo2 = s(photo2)
		item.ShopLogo = s(shopLogo)
		item.UpdateDate = s(updateDate)
		item.Photo1ThumbW640 = s(photo1ThumbW640)
		item.Photo2ThumbW640 = s(photo2ThumbW640)
		item.ShopLogoThumb640x640 = s(shopLogoThumb640x640)
		item.ShopLogoThumbW640 = s(shopLogoThumbW640)

		result[item.ShopID] = item
	}
	return result, nil
}

func shopsEqual(a, b models.ShopItem) bool {
	return a.ShopID == b.ShopID &&
		a.ShopName == b.ShopName &&
		a.ShopNameKana == b.ShopNameKana &&
		a.ShopNameEnglish == b.ShopNameEnglish &&
		a.ShopNameChinaCN == b.ShopNameChinaCN &&
		a.ShopNameChinaTW == b.ShopNameChinaTW &&
		a.ShopNameKorea == b.ShopNameKorea &&
		a.ShopNameFrance == b.ShopNameFrance &&
		a.ShopNameVietnam == b.ShopNameVietnam &&
		a.ShopNameThai == b.ShopNameThai &&
		a.Abbr == b.Abbr &&
		a.WebStatus == b.WebStatus &&
		a.Searches == b.Searches &&
		a.Genre == b.Genre &&
		a.GenreSub == b.GenreSub &&
		a.GenreSubEnglish == b.GenreSubEnglish &&
		a.GenreMemo == b.GenreMemo &&
		a.GenreMemoEnglish == b.GenreMemoEnglish &&
		a.GenreMemoChinaCN == b.GenreMemoChinaCN &&
		a.GenreMemoChinaTW == b.GenreMemoChinaTW &&
		a.GenreMemoKorea == b.GenreMemoKorea &&
		a.GenreMemoFrance == b.GenreMemoFrance &&
		a.GenreMemoVietnam == b.GenreMemoVietnam &&
		a.GenreMemoThai == b.GenreMemoThai &&
		a.GroupID == b.GroupID &&
		a.TenantCode == b.TenantCode &&
		a.Tel == b.Tel &&
		a.UserUrl == b.UserUrl &&
		a.Floor == b.Floor &&
		a.Floors == b.Floors &&
		a.Area == b.Area &&
		a.AreaSub == b.AreaSub &&
		a.Number == b.Number &&
		a.OpenYear == b.OpenYear &&
		a.OpenMonth == b.OpenMonth &&
		a.OpenDay == b.OpenDay &&
		a.CloseFlg == b.CloseFlg &&
		a.PubStart == b.PubStart &&
		a.PubEnd == b.PubEnd &&
		a.OpenTime == b.OpenTime &&
		a.Description == b.Description &&
		a.Qr == b.Qr &&
		a.FoodClass == b.FoodClass &&
		a.Seats == b.Seats &&
		a.Smoking == b.Smoking &&
		a.Reservation == b.Reservation &&
		a.LunchMenu == b.LunchMenu &&
		a.DinnerMenu == b.DinnerMenu &&
		a.TakeOut == b.TakeOut &&
		a.ChildrensMenu == b.ChildrensMenu &&
		a.BabySeat == b.BabySeat &&
		a.Alcohol == b.Alcohol &&
		a.Options == b.Options &&
		a.Photo1 == b.Photo1 &&
		a.Photo2 == b.Photo2 &&
		a.ShopLogo == b.ShopLogo &&
		a.UpdateDate == b.UpdateDate &&
		a.Photo1ThumbW640 == b.Photo1ThumbW640 &&
		a.Photo2ThumbW640 == b.Photo2ThumbW640 &&
		a.ShopLogoThumb640x640 == b.ShopLogoThumb640x640 &&
		a.ShopLogoThumbW640 == b.ShopLogoThumbW640
}

func (m *Manager) loadAllShopNews() (map[string]models.ShopNewsItem, error) {
	query := `SELECT 
		shop_news_id, shop_id, shop_name, shop_logo, shop_floors_name, title, body, categories,
		date_start, date_end, photo1, update_date
		FROM shop_news`

	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.ShopNewsItem)
	for rows.Next() {
		var item models.ShopNewsItem
		var (
			shopNewsId, shopId, shopName, shopLogo, shopFloorsName, title, body, categories,
			dateStart, dateEnd, photo1, updateDate *string
		)

		if err := rows.Scan(
			&shopNewsId, &shopId, &shopName, &shopLogo, &shopFloorsName, &title, &body, &categories,
			&dateStart, &dateEnd, &photo1, &updateDate,
		); err != nil {
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
		item.UpdateDate = s(updateDate)

		result[item.ShopNewsID] = item
	}
	return result, nil
}

func shopNewsEqual(a, b models.ShopNewsItem) bool {
	return a.ShopNewsID == b.ShopNewsID &&
		a.ShopID == b.ShopID &&
		a.ShopName == b.ShopName &&
		a.ShopLogo == b.ShopLogo &&
		a.ShopFloorsName == b.ShopFloorsName &&
		a.Title == b.Title &&
		a.Body == b.Body &&
		a.Categories == b.Categories &&
		a.DateStart == b.DateStart &&
		a.DateEnd == b.DateEnd &&
		a.Photo1 == b.Photo1 &&
		a.UpdateDate == b.UpdateDate
}

func (m *Manager) loadAllEventNews() (map[string]models.EventNewsItem, error) {
	query := `SELECT 
		event_id, title, body, categories, date_start, date_end, display_end, venues,
		photo1, update_date
		FROM event_news`

	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.EventNewsItem)
	for rows.Next() {
		var item models.EventNewsItem
		var (
			eventId, title, body, categories, dateStart, dateEnd, displayEnd, venues,
			photo1, updateDate *string
		)

		if err := rows.Scan(
			&eventId, &title, &body, &categories, &dateStart, &dateEnd, &displayEnd, &venues,
			&photo1, &updateDate,
		); err != nil {
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
		item.UpdateDate = s(updateDate)

		result[item.EventID] = item
	}
	return result, nil
}

func eventNewsEqual(a, b models.EventNewsItem) bool {
	return a.EventID == b.EventID &&
		a.Title == b.Title &&
		a.Body == b.Body &&
		a.Categories == b.Categories &&
		a.DateStart == b.DateStart &&
		a.DateEnd == b.DateEnd &&
		a.DisplayEnd == b.DisplayEnd &&
		a.Venues == b.Venues &&
		a.Photo1 == b.Photo1 &&
		a.UpdateDate == b.UpdateDate
}

func (m *Manager) loadAllSpecials() (map[string]models.SpecialItem, error) {
	query := `SELECT 
		special_id, special_title, title, special_sub_body, category_name,
		shop_id, shop_name, update_date
		FROM specials`

	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.SpecialItem)
	for rows.Next() {
		var item models.SpecialItem
		var (
			specialId, specialTitle, title, specialSubBody, categoryName,
			shopId, shopName, updateDate *string
		)

		if err := rows.Scan(
			&specialId, &specialTitle, &title, &specialSubBody, &categoryName,
			&shopId, &shopName, &updateDate,
		); err != nil {
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

		result[item.SpecialID] = item
	}
	return result, nil
}

func specialsEqual(a, b models.SpecialItem) bool {
	return a.SpecialID == b.SpecialID &&
		a.SpecialTitle == b.SpecialTitle &&
		a.Title == b.Title &&
		a.SpecialSubBody == b.SpecialSubBody &&
		a.CategoryName == b.CategoryName &&
		a.ShopID == b.ShopID &&
		a.ShopName == b.ShopName &&
		a.UpdateDate == b.UpdateDate
}

func (m *Manager) loadAllGenres() (map[string]models.GenreItem, error) {
	query := `SELECT genre_id, genre_name, genre_slug FROM genres`

	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.GenreItem)
	for rows.Next() {
		var item models.GenreItem
		var genreId, genreName, genreSlug *string

		if err := rows.Scan(&genreId, &genreName, &genreSlug); err != nil {
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

		result[item.GenreID] = item
	}
	return result, nil
}

func genresEqual(a, b models.GenreItem) bool {
	return a.GenreID == b.GenreID &&
		a.GenreName == b.GenreName &&
		a.GenreSlug == b.GenreSlug
}

func (m *Manager) syncFloors() (int, error) {
	baseURL := m.Config.APISettings.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	endpoint := baseURL + "floorlist?limit=1000"
	fmt.Printf("Fetching floors from: %s\n", endpoint)
	data, err := m.fetchXML(endpoint)
	if err != nil {
		return 0, err
	}

	var resp models.FloorListResponse
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&resp); err != nil {
		return 0, err
	}

	existingFloors, err := m.loadAllFloors()
	if err != nil {
		fmt.Printf("Warning: Failed to load existing floors: %v. Assuming empty.\n", err)
		existingFloors = make(map[string]models.FloorItem)
	}

	updateCount := 0

	tx, err := m.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO floors (floor_id, floor_name, sort_order) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, item := range resp.Items {
		item.FloorID = strings.TrimSpace(item.FloorID)

		existingItem, exists := existingFloors[item.FloorID]
		needsUpdate := true
		if exists {
			if floorsEqual(item, existingItem) {
				needsUpdate = false
			}
			delete(existingFloors, item.FloorID)
		}

		if needsUpdate {
			updateCount++
			if _, err := stmt.Exec(item.FloorID, item.FloorName, item.SortOrder); err != nil {
				tx.Rollback()
				fmt.Printf("ERROR: Failed to insert floor %s: %v\n", item.FloorID, err)
				return 0, fmt.Errorf("insert floor failed: %w", err)
			}
		}
	}

	deletedCount := 0
	if len(existingFloors) > 0 {
		delStmt, err := tx.Prepare("DELETE FROM floors WHERE floor_id = ?")
		if err != nil {
			return 0, err
		}
		defer delStmt.Close()
		for id := range existingFloors {
			if _, err := delStmt.Exec(id); err == nil {
				deletedCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	if resp.UpdateDateAll != "" {
		m.setLastUpdateDateAll("floors", resp.UpdateDateAll)
	}

	return updateCount + deletedCount, nil
}

func (m *Manager) loadAllFloors() (map[string]models.FloorItem, error) {
	query := `SELECT floor_id, floor_name, sort_order FROM floors`

	rows, err := m.DB.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.FloorItem)
	for rows.Next() {
		var item models.FloorItem
		var floorId, floorName, sortOrder *string

		if err := rows.Scan(&floorId, &floorName, &sortOrder); err != nil {
			continue
		}

		s := func(ptr *string) string {
			if ptr == nil {
				return ""
			}
			return *ptr
		}

		item.FloorID = s(floorId)
		item.FloorName = s(floorName)
		item.SortOrder = s(sortOrder)

		result[item.FloorID] = item
	}
	return result, nil
}

func floorsEqual(a, b models.FloorItem) bool {
	return a.FloorID == b.FloorID &&
		a.FloorName == b.FloorName &&
		a.SortOrder == b.SortOrder
}
