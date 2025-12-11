package main

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/db"
	"bridge-ground/internal/server"
	"bridge-ground/internal/sync"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/zserge/lorca"
)

// Globals
var (
	globalCfg *config.Config
	syncMgr   *sync.Manager
	dbMgr     *db.Manager
)

func main() {
	fmt.Println("Starting BridgeGround (Golang with Lorca)...")

	// 1. Load Config
	var err error
	globalCfg, err = config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v. Using defaults.\n", err)
	}

	// 2. Initialize Managers
	dbMgr = db.NewManager()
	syncMgr = sync.NewManager(globalCfg, dbMgr)

	// 3. Start HTTP Server (Goroutine)
	srv := server.NewServer(globalCfg, dbMgr)
	go func() {
		srv.Start()
	}()

	// 4. Create UI Window
	// Start with a loading message
	// Add --remote-allow-origins=* to fix connection issue with newer Chrome versions
	ui, err := lorca.New("data:text/html,<h1>Loading BridgeGround...</h1>", "", 1000, 800, "--remote-allow-origins=*")
	if err != nil {
		log.Fatal("Failed to start Lorca (Chrome/Edge not found?): ", err)
	}
	defer ui.Close()

	// 5. Bind Go Functions
	// These will be available as window.go_getConfig(), etc. (returning Promises)

	ui.Bind("go_getConfig", func() *config.Config {
		return globalCfg
	})

	ui.Bind("go_saveConfig", func(newCfg config.Config) error {
		if err := config.SaveConfig(&newCfg); err != nil {
			return err
		}
		*globalCfg = newCfg
		// In a real app, you might want to signal the sync manager to reload config
		return nil
	})

	ui.Bind("go_startManualSync", func() map[string]interface{} {
		// This runs in the UI thread/context managed by Lorca.
		// StartSync blocks, so it might freeze the UI if Lorca doesn't handle it concurrently.
		// Lorca uses a message loop. Long operations should ideally be in a goroutine,
		// but we need to return a value to the Promise.
		// If we return a channel, Lorca might not support it directly as Promise.
		// For now, we run it blocking. If UI freezes, we'll need to refactor to async event pattern.
		err := syncMgr.StartSync()
		if err != nil {
			return map[string]interface{}{"success": false, "message": err.Error()}
		}
		return map[string]interface{}{"success": true}
	})

	ui.Bind("go_getDataCounts", func() *db.DataCounts {
		counts, _ := dbMgr.GetDataCounts()
		return counts
	})

	// 6. Setup Progress Callback
	syncMgr.SetProgressCallback(func(p sync.SyncProgress) {
		b, _ := json.Marshal(p)
		// ui.Eval is thread-safe
		jsCode := fmt.Sprintf("if(window.dispatchSyncProgress) window.dispatchSyncProgress(%s)", string(b))
		ui.Eval(jsCode)
	})

	// 7. Auto Sync Logic
	if globalCfg.SyncSettings.SyncOnStartup {
		go func() {
			// Delay to ensure UI is ready to receive progress events
			time.Sleep(3 * time.Second)
			syncMgr.StartSync()
		}()
	}

	if globalCfg.SyncSettings.AutoSyncEnabled {
		go func() {
			ticker := time.NewTicker(time.Duration(globalCfg.SyncSettings.SyncIntervalMinutes) * time.Minute)
			for range ticker.C {
				syncMgr.StartSync()
			}
		}()
	}

	// 8. Navigate to App
	// Wait a bit for server to be up
	time.Sleep(500 * time.Millisecond)
	port := globalCfg.ServerSettings.Port
	url := fmt.Sprintf("http://localhost:%d/index.html", port)
	fmt.Println("Loading URL:", url)
	ui.Load(url)

	// 9. Wait for exit
	<-ui.Done()
}
