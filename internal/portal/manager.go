package portal

import (
	"bridge-ground/internal/config"
	"bridge-ground/internal/logging"
	"bridge-ground/internal/server"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const maxLogBatch = 200

// Manager handles Portal CMS device registration and periodic status reporting.
type Manager struct {
	mu                   sync.Mutex
	cfg                  *config.Config
	apps                 *server.AppRegistry
	saveConfig           func(*config.Config) error
	client               *Client
	startTime            time.Time
	rtdb                 *rtdbClient
	rtdbScreenshotWatched map[string]struct{} // deviceIDs already subscribed for screenshots
	rtdbLogWatched        map[string]struct{} // deviceIDs already subscribed for logs
}

// NewManager creates a portal Manager.
func NewManager(cfg *config.Config, apps *server.AppRegistry, saveConfig func(*config.Config) error) *Manager {
	return &Manager{
		cfg:                   cfg,
		apps:                  apps,
		saveConfig:            saveConfig,
		startTime:             time.Now(),
		rtdbScreenshotWatched: make(map[string]struct{}),
		rtdbLogWatched:        make(map[string]struct{}),
	}
}

// Start begins the registration, approval-polling, and heartbeat loop.
// Returns immediately if workerBaseUrl or registrationToken are not configured.
func (m *Manager) Start() {
	ps := &m.cfg.PortalSettings
	if ps.WorkerBaseURL == "" || ps.RegistrationToken == "" {
		logging.Info("PORTAL", "Not configured (workerBaseUrl or registrationToken missing)")
		return
	}
	m.client = NewClient(ps.WorkerBaseURL)

	if ps.FirebaseDatabaseURL != "" {
		m.rtdb = newRTDBClient(ps.FirebaseDatabaseURL)
	}

	m.registerSelf()

	interval := ps.StatusReportIntervalSecs
	if interval <= 0 {
		interval = 900
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Approval-polling runs independently so newly registered devices pick up
	// credentials as soon as the admin approves them.
	go m.approvalPollLoop()

	// Subscribe to RTDB signals for already-approved devices.
	m.subscribeApprovedDevices()

	// Run immediately on start so already-approved devices report online right away.
	m.registerNewApps()
	m.sendHeartbeat()

	for range ticker.C {
		m.registerNewApps()
		m.sendHeartbeat()
	}
}

// registerSelf registers Bridge-Ground itself if not already registered.
func (m *Manager) registerSelf() {
	hostname, _ := os.Hostname()

	m.mu.Lock()
	existing := m.findDevice("Bridge-Ground", hostname)
	m.mu.Unlock()

	if existing != nil {
		return
	}

	resp, err := m.client.Register(m.cfg.PortalSettings.RegistrationToken, RegisterRequest{
		AppName:  "Bridge-Ground",
		Hostname: hostname,
		IP:       localIP(),
	})
	if err != nil {
		logging.Error("PORTAL", fmt.Sprintf("Self-registration failed: %v", err))
		return
	}

	m.mu.Lock()
	m.upsertDevice(config.PortalDevice{
		AppName:   "Bridge-Ground",
		Hostname:  hostname,
		PendingID: resp.PendingID,
	})
	m.mu.Unlock()

	if err := m.saveConfig(m.cfg); err != nil {
		logging.Warn("PORTAL", fmt.Sprintf("Failed to save config after self-registration: %v", err))
	}
	logging.Info("PORTAL", fmt.Sprintf("Registered self as Bridge-Ground (%s), pendingId=%s", hostname, resp.PendingID))
}

// registerNewApps registers any apps that the AppRegistry reports but that are
// not yet in the portal config.
func (m *Manager) registerNewApps() {
	for _, app := range m.apps.List() {
		m.mu.Lock()
		existing := m.findDevice(app.Name, app.Hostname)
		m.mu.Unlock()

		if existing != nil {
			continue
		}

		ip := app.IP
		if ip == "" {
			ip = localIP()
		}

		resp, err := m.client.Register(m.cfg.PortalSettings.RegistrationToken, RegisterRequest{
			AppName:  app.Name,
			Hostname: app.Hostname,
			IP:       ip,
		})
		if err != nil {
			logging.Error("PORTAL", fmt.Sprintf("Registration failed for %s (%s): %v", app.Name, app.Hostname, err))
			continue
		}

		m.mu.Lock()
		m.upsertDevice(config.PortalDevice{
			AppName:   app.Name,
			Hostname:  app.Hostname,
			PendingID: resp.PendingID,
		})
		m.mu.Unlock()

		if err := m.saveConfig(m.cfg); err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Failed to save config after app registration: %v", err))
		}
		logging.Info("PORTAL", fmt.Sprintf("Registered app %s (%s), pendingId=%s", app.Name, app.Hostname, resp.PendingID))
	}
}

// approvalPollLoop polls GET /v1/device for every device that has a PendingID but no
// DeviceID yet. It backs off to every 60 seconds once all devices are approved.
func (m *Manager) approvalPollLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
		copy(devices, m.cfg.PortalSettings.Devices)
		m.mu.Unlock()

		changed := false
		for _, d := range devices {
			if d.PendingID == "" || d.DeviceID != "" {
				continue
			}
			resp, err := m.client.PollForApproval(m.cfg.PortalSettings.RegistrationToken, d.PendingID)
			if err != nil {
				logging.Warn("PORTAL", fmt.Sprintf("Approval poll failed for %s (%s): %v", d.AppName, d.Hostname, err))
				continue
			}
			if resp.Status != "approved" {
				continue
			}

			m.mu.Lock()
			entry := m.findDevice(d.AppName, d.Hostname)
			if entry != nil {
				entry.DeviceID    = resp.DeviceID
				entry.DeviceToken = resp.DeviceToken
			}
			m.mu.Unlock()

			changed = true
			logging.Info("PORTAL", fmt.Sprintf("Approval received for %s (%s): deviceId=%s", d.AppName, d.Hostname, resp.DeviceID))
		}

		if changed {
			if err := m.saveConfig(m.cfg); err != nil {
				logging.Warn("PORTAL", fmt.Sprintf("Failed to save config after approval: %v", err))
			}
			go m.sendHeartbeat()
			m.subscribeApprovedDevices()
		}
	}
}

// subscribeApprovedDevices starts RTDB SSE subscriptions for any approved device
// that hasn't been subscribed yet. Safe to call multiple times (idempotent).
func (m *Manager) subscribeApprovedDevices() {
	if m.rtdb == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.cfg.PortalSettings.Devices {
		if d.DeviceID == "" {
			continue
		}
		// Screenshot requests: only for non-BG devices (BG itself is never a screenshot target).
		if d.AppName != "Bridge-Ground" {
			if _, already := m.rtdbScreenshotWatched[d.DeviceID]; !already {
				m.rtdb.Subscribe("screenshot-requests", d.DeviceID, func(id, _ string) { m.handleScreenshotSignal(id) })
				m.rtdbScreenshotWatched[d.DeviceID] = struct{}{}
				logging.Info("RTDB", fmt.Sprintf("Subscribed to screenshot signals for %s (%s)", d.AppName, d.DeviceID))
			}
		}
		// Log requests: all devices including BG itself.
		if _, already := m.rtdbLogWatched[d.DeviceID]; !already {
			m.rtdb.Subscribe("log-requests", d.DeviceID, m.handleLogSignal)
			m.rtdbLogWatched[d.DeviceID] = struct{}{}
			logging.Info("RTDB", fmt.Sprintf("Subscribed to log signals for %s (%s)", d.AppName, d.DeviceID))
		}
	}
}

// handleScreenshotSignal is called by rtdbClient when a screenshot signal arrives.
func (m *Manager) handleScreenshotSignal(deviceID string) {
	// Delete the RTDB signal immediately so it does not re-trigger on reconnect.
	m.rtdb.Delete("screenshot-requests", deviceID)

	m.mu.Lock()
	bg := m.selfDevice()
	if bg == nil || bg.DeviceToken == "" {
		m.mu.Unlock()
		return
	}
	bgToken := bg.DeviceToken
	devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	go m.uploadScreenshotForDevice(bgToken, deviceID, devices)
}

// handleLogSignal is called by rtdbClient when a log request signal arrives.
// date is an optional YYYY-MM-DD string from the signal payload (empty = today).
func (m *Manager) handleLogSignal(deviceID, date string) {
	m.rtdb.Delete("log-requests", deviceID)

	m.mu.Lock()
	devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	go m.uploadLogsForDevice(deviceID, date, devices)
}

// uploadLogsForDevice reads log entries for the given date (YYYY-MM-DD; empty = today)
// and writes them to RTDB logs/{deviceID}.
func (m *Manager) uploadLogsForDevice(deviceID, date string, devices []config.PortalDevice) {
	var target *config.PortalDevice
	for i := range devices {
		if devices[i].DeviceID == deviceID {
			target = &devices[i]
			break
		}
	}
	if target == nil {
		return
	}

	now := time.Now()
	fromDate := date
	toDate   := date
	if fromDate == "" {
		fromDate = now.Format("2006-01-02")
		toDate   = fromDate
	}

	var entries []logging.LogEntry
	if target.AppName == "Bridge-Ground" {
		entries = logging.GetEntriesForRange(fromDate, toDate)
	} else {
		app, ok := m.findAppInfo(target.AppName, target.Hostname)
		if !ok || app.LogDir == "" || app.LogPrefix == "" {
			// Write empty result so the portal knows the request was handled.
			_ = m.rtdb.Put("logs", deviceID, map[string]interface{}{"entries": []interface{}{}, "at": now.UnixMilli()})
			return
		}
		entries = logging.ReadAppLogsFromDir(app.LogDir, app.LogPrefix, fromDate, toDate)
	}

	if len(entries) > maxLogBatch {
		entries = entries[len(entries)-maxLogBatch:]
	}

	logEntries := make([]LogEntry, len(entries))
	for i, e := range entries {
		logEntries[i] = LogEntry{Timestamp: e.Timestamp, Level: e.Level, Tag: e.Tag, Message: e.Message}
	}

	payload := map[string]interface{}{
		"entries": logEntries,
		"at":      now.UnixMilli(),
	}

	if err := m.rtdb.Put("logs", deviceID, payload); err != nil {
		logging.Warn("PORTAL", fmt.Sprintf("Log upload failed for %s: %v", target.AppName, err))
	} else {
		logging.Info("PORTAL", fmt.Sprintf("Logs uploaded for %s (%s): %d entries", target.AppName, target.Hostname, len(entries)))
	}
}

// sendHeartbeat sends one batched heartbeat for all approved devices and handles
// screenshot commands returned by the Worker.
func (m *Manager) sendHeartbeat() {
	m.mu.Lock()
	bg := m.selfDevice()
	if bg == nil || bg.DeviceID == "" || bg.DeviceToken == "" {
		m.mu.Unlock()
		return // not yet approved
	}
	bgToken  := bg.DeviceToken
	devices  := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	metrics    := CollectMetrics()
	uptimeSecs := int(time.Since(m.startTime).Seconds())
	appList    := m.apps.List()
	currentIP  := localIP()

	var entries []DeviceStatusEntry
	for _, d := range devices {
		if d.DeviceID == "" {
			continue
		}
		entry := m.buildStatusEntry(d, metrics, uptimeSecs, appList, currentIP)
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return
	}

	heartbeatResp, err := m.client.Heartbeat(bgToken, HeartbeatRequest{Devices: entries})
	if err != nil {
		logging.Warn("PORTAL", fmt.Sprintf("Heartbeat failed: %v", err))
		return
	}

	// Handle screenshot commands
	for deviceID, cmd := range heartbeatResp.Commands {
		if !cmd.Screenshot {
			continue
		}
		go m.uploadScreenshotForDevice(bgToken, deviceID, devices)
	}
}

func (m *Manager) buildStatusEntry(d config.PortalDevice, metrics Metrics, uptimeSecs int, apps []server.AppInfo, currentIP string) DeviceStatusEntry {
	if d.AppName == "Bridge-Ground" {
		return DeviceStatusEntry{
			DeviceID:    d.DeviceID,
			Status:      "online",
			IP:          currentIP,
			CPU:         metrics.CPU,
			Memory:      metrics.Memory,
			Temperature: metrics.Temperature,
			Storage:     metrics.Storage,
			Uptime:      uptimeSecs,
		}
	}

	status := "offline"
	var appUptimeSecs int
	var appIP string
	for _, app := range apps {
		if app.Name == d.AppName && app.Hostname == d.Hostname {
			if app.Online {
				status = "online"
			}
			if app.StartedAt != nil {
				appUptimeSecs = int(time.Since(*app.StartedAt).Seconds())
			}
			appIP = app.IP
			break
		}
	}
	if appIP == "" {
		appIP = currentIP
	}
	return DeviceStatusEntry{
		DeviceID:    d.DeviceID,
		Status:      status,
		IP:          appIP,
		CPU:         metrics.CPU,
		Memory:      metrics.Memory,
		Temperature: metrics.Temperature,
		Storage:     metrics.Storage,
		Uptime:      appUptimeSecs,
	}
}

// uploadScreenshotForDevice finds the app for the given deviceID, captures a screenshot,
// and uploads it to the portal. Called from a goroutine.
func (m *Manager) uploadScreenshotForDevice(bgToken, deviceID string, devices []config.PortalDevice) {
	var target *config.PortalDevice
	for i := range devices {
		if devices[i].DeviceID == deviceID {
			target = &devices[i]
			break
		}
	}
	if target == nil || target.AppName == "Bridge-Ground" {
		return
	}

	app, ok := m.findAppInfo(target.AppName, target.Hostname)
	if !ok {
		return
	}

	var data []byte
	captured, captureErr := server.CaptureAppScreen(app.Name)
	if captureErr != nil || len(captured) == 0 {
		cached, _, _, hasCache := m.apps.GetScreenshot(app.ID)
		if !hasCache || len(cached) == 0 {
			logging.Warn("PORTAL", fmt.Sprintf("No screenshot available for %s: %v", target.AppName, captureErr))
			return
		}
		data = cached
	} else {
		m.apps.StoreScreenshot(app.ID, captured, "image/jpeg")
		data = captured
	}

	if err := m.client.UploadScreenshot(bgToken, deviceID, data); err != nil {
		logging.Warn("PORTAL", fmt.Sprintf("Screenshot upload failed for %s: %v", target.AppName, err))
	} else {
		logging.Info("PORTAL", fmt.Sprintf("Screenshot uploaded for %s (%s)", target.AppName, target.Hostname))
	}
}

// selfDevice returns BG's own PortalDevice entry. Must be called with m.mu held.
func (m *Manager) selfDevice() *config.PortalDevice {
	hostname, _ := os.Hostname()
	return m.findDevice("Bridge-Ground", hostname)
}

// findDevice returns a pointer to the matching device entry, or nil.
// Must be called with m.mu held.
func (m *Manager) findDevice(appName, hostname string) *config.PortalDevice {
	for i := range m.cfg.PortalSettings.Devices {
		d := &m.cfg.PortalSettings.Devices[i]
		if d.AppName == appName && d.Hostname == hostname {
			return d
		}
	}
	return nil
}

// upsertDevice adds or updates a device entry.
// Must be called with m.mu held.
func (m *Manager) upsertDevice(d config.PortalDevice) {
	for i := range m.cfg.PortalSettings.Devices {
		e := &m.cfg.PortalSettings.Devices[i]
		if e.AppName == d.AppName && e.Hostname == d.Hostname {
			if d.PendingID != "" {
				e.PendingID = d.PendingID
			}
			if d.DeviceID != "" {
				e.DeviceID = d.DeviceID
			}
			if d.DeviceToken != "" {
				e.DeviceToken = d.DeviceToken
			}
			return
		}
	}
	m.cfg.PortalSettings.Devices = append(m.cfg.PortalSettings.Devices, d)
}

// findAppInfo returns the AppInfo whose (Name, Hostname) matches.
func (m *Manager) findAppInfo(appName, hostname string) (server.AppInfo, bool) {
	for _, app := range m.apps.List() {
		if app.Name == appName && app.Hostname == hostname {
			return app, true
		}
	}
	return server.AppInfo{}, false
}

// localIP returns this machine's preferred outbound IP address.
func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "0.0.0.0"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
