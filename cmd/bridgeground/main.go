package main

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/server"
	appSync "bridge-ground/internal/sync"
	"encoding/json"
	"fmt"
	"log"
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
	fmt.Printf("Starting BridgeGround v%s (Golang with Lorca)...\n", config.Version)

	// 1. Load Config
	var err error
	globalCfg, err = config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v. Using defaults.\n", err)
	}

	// 2. Initialize Managers
	dbMgr = db.NewManager()
	if err := dbMgr.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbMgr.Close()

	syncMgr = appSync.NewManager(globalCfg, dbMgr)

	// 3. Start HTTP Server (Goroutine)
	srv = server.NewServer(globalCfg, dbMgr)
	go func() {
		srv.Start()
	}()

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

	// 5. Auto Sync Logic
	if globalCfg.SyncSettings.SyncOnStartup {
		go func() {
			// Delay slightly to let server start
			time.Sleep(3 * time.Second)
			if updated, err := syncMgr.StartSync(); err == nil && updated {
				srv.BroadcastEvent("update", map[string]interface{}{
					"type":      "update",
					"timestamp": time.Now().Format(time.RFC3339),
					"message":   "Startup sync completed",
				})
			}
		}()
	}

	if globalCfg.SyncSettings.AutoSyncEnabled {
		go func() {
			ticker := time.NewTicker(time.Duration(globalCfg.SyncSettings.SyncIntervalMinutes) * time.Minute)
			for range ticker.C {
				if updated, err := syncMgr.StartSync(); err == nil && updated {
					srv.BroadcastEvent("update", map[string]interface{}{
						"type":      "update",
						"timestamp": time.Now().Format(time.RFC3339),
						"message":   "Auto sync completed",
					})
				}
			}
		}()
	}

	// 6. Start System Tray (Blocking)
	// This will block main thread until systray.Quit() is called
	systray.Run(onReady, onExit)
}

func onReady() {
	// Try to load icon
	// Look for icon in standard locations
	iconPath := "src/assets/icon.ico"
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		// Try finding it relative to executable if not in current dir
		if exePath, err := os.Executable(); err == nil {
			// Assume standard deployment structure: root/Bridge Ground.exe, root/src/assets/...
			// Or root/bin/exe, root/src...
			// Let's try simple relative path first
			_ = exePath // Prevent unused variable error
		}
	}

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

	// Create UI Window
	var err error
	// Use --remote-allow-origins=* to fix connection issue with newer Chrome versions
	newUI, err := lorca.New("data:text/html,<h1>Loading BridgeGround...</h1>", "", 1000, 800, "--remote-allow-origins=*")
	if err != nil {
		log.Println("Failed to start Lorca (Chrome/Edge not found?):", err)
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

		// Update startup settings
		if err := updateStartupRegistry(newCfg.SystemSettings.RunOnStartup); err != nil {
			fmt.Printf("Failed to update startup registry: %v\n", err)
		}

		*globalCfg = newCfg
		return nil
	})

	newUI.Bind("go_startManualSync", func() map[string]interface{} {
		updated, err := syncMgr.StartSync()
		if err != nil {
			return map[string]interface{}{"success": false, "message": err.Error()}
		}

		if updated {
			srv.BroadcastEvent("update", map[string]interface{}{
				"type":      "update",
				"timestamp": time.Now().Format(time.RFC3339),
				"message":   "Manual sync completed",
			})
		}

		return map[string]interface{}{"success": true}
	})

	newUI.Bind("go_getDataCounts", func() *db.DataCounts {
		counts, _ := dbMgr.GetDataCounts()
		return counts
	})

	// Load App URL
	port := globalCfg.ServerSettings.Port
	url := fmt.Sprintf("http://localhost:%d/index.html", port)
	newUI.Load(url)

	ui = newUI

	// Watch for UI close
	go func(u lorca.UI) {
		<-u.Done()
		uiMutex.Lock()
		if ui == u {
			ui = nil
		}
		uiMutex.Unlock()
		// Do NOT call systray.Quit() here, as we want to keep running in background
	}(newUI)
}
