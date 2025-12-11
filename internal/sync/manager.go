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

// StartSync executes the full synchronization process
func (m *Manager) StartSync() error {
	fmt.Println("--- Starting Data Synchronization (Go) ---")

	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 0, Message: "開始中..."},
	})
	
	if err := m.DB.Connect(); err != nil {
		m.notifyProgress(SyncProgress{
			Main: &ProgressDetail{Percentage: 0, Message: "DB接続エラー"},
		})
		return err
	}
	defer m.DB.Close()

	if err := m.DB.InitializeSchema(); err != nil {
		return err
	}

	// 1. Sync Shops
	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 10, Message: "店舗データを同期中..."},
	})
	if err := m.syncShops(); err != nil {
		fmt.Printf("Error syncing shops: %v\n", err)
	}

	// 2. Sync Shop News
	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 30, Message: "ショップニュースを同期中..."},
	})
	if err := m.syncShopNews(); err != nil {
		fmt.Printf("Error syncing shop news: %v\n", err)
	}

	// 3. Sync Event News
	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 50, Message: "イベントニュースを同期中..."},
	})
	if err := m.syncEventNews(); err != nil {
		fmt.Printf("Error syncing event news: %v\n", err)
	}

	// 4. Sync Specials
	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 70, Message: "特集データを同期中..."},
	})
	if err := m.syncSpecials(); err != nil {
		fmt.Printf("Error syncing specials: %v\n", err)
	}

	// 5. Sync Genres
	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 90, Message: "ジャンルデータを同期中..."},
	})
	if err := m.syncGenres(); err != nil {
		fmt.Printf("Error syncing genres: %v\n", err)
	}

	fmt.Println("--- Synchronization Completed ---")
	
	// Get stats for final update
	counts, _ := m.DB.GetDataCounts()
	stats := map[string]int{
		"shops":     counts.Shops,
		"shopNews":  counts.ShopNews,
		"eventNews": counts.EventNews,
		"specials":  counts.Specials,
	}

	m.notifyProgress(SyncProgress{
		Main: &ProgressDetail{Percentage: 100, Message: "同期完了", Stats: stats},
	})
	return nil
}

// downloadWorker processes download jobs from the channel
func (m *Manager) downloadWorker(id int, jobs <-chan DownloadJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		if err := m.downloadFile(job.RemoteURL, job.LocalPath); err != nil {
			// Log error but don't stop worker
			// fmt.Printf("[Worker %d] Failed to download %s: %v\n", id, job.RemoteURL, err)
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
	// Assuming the API base URL + relative path structure matches the XML content
	// If the XML content has /api/ prefix, and base has it too, we need care.
	// Typically this logic needs to be robust.
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
		go m.downloadWorker(i, jobChan, &wg)
	}

	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)
	wg.Wait()
}

// --- Sync Implementations ---

func (m *Manager) syncShops() error {
	endpoint := m.Config.APISettings.BaseURL + "shoplist"
	data, err := m.fetchXML(endpoint)
	if err != nil { return err }

	var resp models.ShopListResponse
	if err := xml.Unmarshal(data, &resp); err != nil { return err }

	tx, err := m.DB.Conn.Begin()
	if err != nil { return err }
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO shops (shop_id, shop_name, shop_name_kana, shop_name_english, genre, genre_sub, tel, open_time, floor, photo1_remote_url, photo1_local_path, shop_logo_remote_url, shop_logo_local_path, update_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil { return err }
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
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
		stmt.Exec(item.ShopID, item.ShopName, item.ShopNameKana, item.ShopNameEnglish, item.Genre, item.GenreSub, item.Tel, item.OpenTime, item.Floor, item.Photo1RemoteURL, item.Photo1LocalPath, item.ShopLogoRemoteURL, item.ShopLogoLocalPath, item.UpdateDate)
	}
	if err := tx.Commit(); err != nil { return err }
	
	m.processDownloads(downloadJobs)
	return nil
}

func (m *Manager) syncShopNews() error {
	endpoint := m.Config.APISettings.BaseURL + "shopnewslist"
	data, err := m.fetchXML(endpoint)
	if err != nil { return err }

	var resp models.ShopNewsResponse
	if err := xml.Unmarshal(data, &resp); err != nil { return err }

	tx, err := m.DB.Conn.Begin()
	if err != nil { return err }
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO shop_news (shop_news_id, shop_id, title, body, photo1_remote_url, photo1_local_path, update_date) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil { return err }
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		stmt.Exec(item.ShopNewsID, item.ShopID, item.Title, item.Body, item.Photo1RemoteURL, item.Photo1LocalPath, item.UpdateDate)
	}
	if err := tx.Commit(); err != nil { return err }

	m.processDownloads(downloadJobs)
	return nil
}

func (m *Manager) syncEventNews() error {
	endpoint := m.Config.APISettings.BaseURL + "eventnewslist"
	data, err := m.fetchXML(endpoint)
	if err != nil { return err }

	var resp models.EventNewsResponse
	if err := xml.Unmarshal(data, &resp); err != nil { return err }

	tx, err := m.DB.Conn.Begin()
	if err != nil { return err }
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO event_news (event_id, title, body, date_start, date_end, photo1_remote_url, photo1_local_path, update_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil { return err }
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	for _, item := range resp.Items {
		if item.Photo1 != "" {
			item.Photo1RemoteURL = m.resolveURL(item.Photo1)
			item.Photo1LocalPath = m.resolveLocalPath(baseFileDir, item.Photo1)
			downloadJobs = append(downloadJobs, DownloadJob{item.Photo1RemoteURL, item.Photo1LocalPath})
		}
		stmt.Exec(item.EventID, item.Title, item.Body, item.DateStart, item.DateEnd, item.Photo1RemoteURL, item.Photo1LocalPath, item.UpdateDate)
	}
	if err := tx.Commit(); err != nil { return err }

	m.processDownloads(downloadJobs)
	return nil
}

func (m *Manager) syncSpecials() error {
	endpoint := m.Config.APISettings.BaseURL + "speciallist"
	data, err := m.fetchXML(endpoint)
	if err != nil { return err }

	var resp models.SpecialListResponse
	if err := xml.Unmarshal(data, &resp); err != nil { return err }

	tx, err := m.DB.Conn.Begin()
	if err != nil { return err }
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO specials (special_id, special_title, title, special_sub_body, category_name, shop_id, shop_name, update_date, special_image_local_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil { return err }
	defer stmt.Close()

	baseFileDir := m.getBaseFileDir()
	var downloadJobs []DownloadJob

	// Specials are nested in specialTitle items
	for _, parent := range resp.Items {
		// Iterate over sub-items
		for _, item := range parent.Items {
			if item.Type != "special" { continue }

			// Use parent properties where appropriate
			item.SpecialTitle = parent.SpecialTitle
			item.UpdateDate = parent.UpdateDate

			if item.SpecialImage != "" {
				item.SpecialImageRemoteURL = m.resolveURL(item.SpecialImage)
				item.SpecialImageLocalPath = m.resolveLocalPath(baseFileDir, item.SpecialImage)
				downloadJobs = append(downloadJobs, DownloadJob{item.SpecialImageRemoteURL, item.SpecialImageLocalPath})
			}
			stmt.Exec(item.SpecialID, item.SpecialTitle, item.Title, item.SpecialSubBody, item.CategoryName, item.ShopID, item.ShopName, item.UpdateDate, item.SpecialImageLocalPath)
		}
	}
	if err := tx.Commit(); err != nil { return err }

	m.processDownloads(downloadJobs)
	return nil
}

func (m *Manager) syncGenres() error {
	endpoint := m.Config.APISettings.BaseURL + "genrelist"
	data, err := m.fetchXML(endpoint)
	if err != nil { return err }

	var resp models.GenreListResponse
	if err := xml.Unmarshal(data, &resp); err != nil { return err }

	tx, err := m.DB.Conn.Begin()
	if err != nil { return err }
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO genres (genre_id, genre_name, genre_slug) VALUES (?, ?, ?)`)
	if err != nil { return err }
	defer stmt.Close()

	for _, item := range resp.Items {
		stmt.Exec(item.GenreID, item.GenreName, item.GenreSlug)
	}
	return tx.Commit()
}

func (m *Manager) fetchXML(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
