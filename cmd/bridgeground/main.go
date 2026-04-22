package main

//go:generate goversioninfo -icon=../../src/assets/icon.ico

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/logging"
	"bridge-ground/internal/server"
	appSync "bridge-ground/internal/sync"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/zserge/lorca"
)

// Globals
var (
	globalCfg *config.Config
	syncMgr   *appSync.Manager
	dbMgr     *db.Manager
	srv       *server.Server
	ui        lorca.UI
	uiMutex   sync.Mutex
)

func main() {
	// Initialize file logging before anything else.
	// Redirects Go's standard logger (log.Printf etc.) to both stderr and the log file.
	logging.CleanupOldLogs(30)
	if err := logging.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
	}
	defer logging.Close()

	logging.Info("SYSTEM", fmt.Sprintf("Starting BridgeGround v%s", config.Version))

	// 1. Load Config
	var err error
	globalCfg, err = config.LoadConfig()
	if err != nil {
		logging.Warn("CONFIG", fmt.Sprintf("Failed to load config: %v — using defaults", err))
	} else {
		logging.Info("CONFIG", "Config loaded successfully")
	}

	// 2. Initialize Managers
	dbMgr = db.NewManager()
	if err := dbMgr.Connect(); err != nil {
		logging.Fatal("DB", fmt.Sprintf("Failed to connect to database: %v", err))
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbMgr.Close()
	logging.Info("DB", "Database connected")

	syncMgr = appSync.NewManager(globalCfg, dbMgr)

	// 3. Start HTTP Server (Goroutine)
	srv = server.NewServer(globalCfg, dbMgr)
	go func() {
		srv.Start()
	}()
	logging.Info("SERVER", fmt.Sprintf("HTTP server starting on port %d", globalCfg.ServerSettings.Port))

	// 4. Setup Progress Callback (Thread-safe)
	syncMgr.SetProgressCallback(func(p appSync.SyncProgress) {
		b, _ := json.Marshal(p)
		jsCode := fmt.Sprintf("if(window.dispatchSyncProgress) window.dispatchSyncProgress(%s)", string(b))

		uiMutex.Lock()
		if ui != nil {
			// ui.Eval might fail if window is closed concurrently, ignore error
			_ = ui.Eval(jsCode)
		}
		uiMutex.Unlock()
	})

	syncMgr.SetDataUpdateCallback(func(dataType string) {
		var data interface{}
		var err error

		switch dataType {
		case "shops":
			data, err = srv.FetchShopList()
		case "shop_news":
			data, err = srv.FetchShopNews()
		case "event_news":
			data, err = srv.FetchEventNews()
		case "specials":
			data, err = srv.FetchSpecials()
		case "genres":
			data, err = srv.FetchGenres()
		default:
			logging.Warn("SSE", fmt.Sprintf("Unknown data type updated: %s", dataType))
			return
		}

		if err != nil {
			logging.Error("SSE", fmt.Sprintf("Failed to fetch updated data for %s: %v", dataType, err))
			return
		}

		logging.Info("SSE", fmt.Sprintf("Broadcasting %s update to SSE clients", dataType))
		srv.BroadcastEvent(dataType, map[string]interface{}{
			"type":      dataType,
			"timestamp": time.Now().Format(time.RFC3339),
			"data":      data,
		})
	})

	// 5. Auto Sync Logic
	if globalCfg.SyncSettings.SyncOnStartup {
		go func() {
			// Delay slightly to let server start
			time.Sleep(3 * time.Second)
			logging.Info("SYNC", "Starting startup sync")
			syncMgr.StartSync()
		}()
	}

	if globalCfg.SyncSettings.AutoSyncEnabled {
		logging.Info("SYNC", fmt.Sprintf("Auto-sync enabled, interval: %d minutes", globalCfg.SyncSettings.SyncIntervalMinutes))
		go func() {
			ticker := time.NewTicker(time.Duration(globalCfg.SyncSettings.SyncIntervalMinutes) * time.Minute)
			for range ticker.C {
				logging.Info("SYNC", "Starting scheduled sync")
				syncMgr.StartSync()
			}
		}()
	}

	// 6. Start System Tray (Blocking)
	// This will block main thread until systray.Quit() is called
	systray.Run(onReady, onExit)
}

func onReady() {
	iconPath := "src/assets/icon.ico"
	iconData, err := os.ReadFile(iconPath)
	if err == nil {
		systray.SetIcon(iconData)
	} else {
		systray.SetTitle("BG")
	}

	systray.SetTooltip("BridgeGround Server")

	mOpen := systray.AddMenuItem("設定画面を開く", "設定画面を表示します")
	mQuit := systray.AddMenuItem("終了", "アプリケーションを終了します")

	// Event loop for tray items
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openUI()
			case <-mQuit.ClickedCh:
				logging.Info("SYSTEM", "Quit requested via system tray")
				systray.Quit()
			}
		}
	}()

	// Initial UI state
	if !globalCfg.SystemSettings.StartHidden {
		openUI()
	}
}

func onExit() {
	logging.Info("SYSTEM", "BridgeGround shutting down")
	uiMutex.Lock()
	if ui != nil {
		ui.Close()
	}
	uiMutex.Unlock()
}

func openUI() {
	uiMutex.Lock()
	defer uiMutex.Unlock()

	if ui != nil {
		// Window is already open
		return
	}

	var err error
	newUI, err := lorca.New("data:text/html,<h1>Loading BridgeGround...</h1>", "", 1000, 800, "--remote-allow-origins=*")
	if err != nil {
		logging.Error("UI", fmt.Sprintf("Failed to start Lorca (Chrome/Edge not found?): %v", err))
		return
	}

	// Bind Go Functions
	newUI.Bind("go_getConfig", func() *config.Config {
		return globalCfg
	})

	newUI.Bind("go_getAppVersion", func() string {
		return config.Version
	})

	newUI.Bind("go_saveConfig", func(newCfg config.Config) error {
		if err := config.SaveConfig(&newCfg); err != nil {
			return err
		}
		if err := updateStartupRegistry(newCfg.SystemSettings.RunOnStartup); err != nil {
			logging.Warn("CONFIG", fmt.Sprintf("Failed to update startup registry: %v", err))
		}
		*globalCfg = newCfg
		logging.Info("CONFIG", "Config saved")
		return nil
	})

	newUI.Bind("go_startManualSync", func() map[string]interface{} {
		logging.Info("SYNC", "Manual sync started via UI")
		_, err := syncMgr.StartSync()
		if err != nil {
			logging.Error("SYNC", fmt.Sprintf("Manual sync failed: %v", err))
			return map[string]interface{}{"success": false, "message": err.Error()}
		}
		logging.Info("SYNC", "Manual sync completed")
		return map[string]interface{}{"success": true}
	})

	newUI.Bind("go_getDataCounts", func() *db.DataCounts {
		counts, _ := dbMgr.GetDataCounts()
		return counts
	})

	// Load App URL
	port := globalCfg.ServerSettings.Port
	url := fmt.Sprintf("http://localhost:%d/index.html", port)

	if !waitForServer(url, 10*time.Second) {
		logging.Warn("SERVER", fmt.Sprintf("Server did not respond within timeout at %s", url))
	}

	newUI.Load(url)
	logging.Info("UI", fmt.Sprintf("UI window opened: %s", url))

	ui = newUI

	// Watch for UI close
	go func(u lorca.UI) {
		<-u.Done()
		uiMutex.Lock()
		if ui == u {
			ui = nil
		}
		uiMutex.Unlock()
		logging.Info("UI", "UI window closed")
	}(newUI)
}

func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
