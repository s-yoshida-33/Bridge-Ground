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
	mu            sync.Mutex
	cfg           *config.Config
	apps          *server.AppRegistry
	saveConfig    func(*config.Config) error
	client        *Client
	startTime     time.Time
	lastLogSentAt time.Time
}

// NewManager creates a portal Manager.
func NewManager(cfg *config.Config, apps *server.AppRegistry, saveConfig func(*config.Config) error) *Manager {
	return &Manager{
		cfg:        cfg,
		apps:       apps,
		saveConfig: saveConfig,
		startTime:  time.Now(),
	}
}

// Start begins the registration and periodic status-reporting loop.
// Returns immediately if workerBaseUrl or registrationToken are not configured.
func (m *Manager) Start() {
	ps := &m.cfg.PortalSettings
	if ps.WorkerBaseURL == "" || ps.RegistrationToken == "" {
		logging.Info("PORTAL", "Not configured (workerBaseUrl or registrationToken missing)")
		return
	}
	m.client = NewClient(ps.WorkerBaseURL)

	m.registerSelf()

	interval := ps.StatusReportIntervalSecs
	if interval <= 0 {
		interval = 60
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.registerNewApps()
		m.reportAllStatus()
		m.sendAllLogs()
		m.checkAndUploadScreenshots()
	}
}

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

func (m *Manager) reportAllStatus() {
	metrics    := CollectMetrics()
	uptimeSecs := int(time.Since(m.startTime).Seconds())
	appList    := m.apps.List()
	currentIP  := localIP()

	m.mu.Lock()
	devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	for _, d := range devices {
		if d.DeviceID == "" || d.DeviceToken == "" {
			continue
		}
		req := m.buildStatusRequest(d, metrics, uptimeSecs, appList, currentIP)
		if err := m.client.ReportStatus(d.DeviceToken, req); err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Status report failed for %s/%s: %v", d.AppName, d.Hostname, err))
		}
	}
}

func (m *Manager) buildStatusRequest(d config.PortalDevice, metrics Metrics, uptimeSecs int, apps []server.AppInfo, currentIP string) StatusRequest {
	if d.AppName == "Bridge-Ground" {
		return StatusRequest{
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
	return StatusRequest{
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

// sendAllLogs collects new log entries since the last send and pushes them to the portal.
func (m *Manager) sendAllLogs() {
	now  := time.Now()
	from := m.lastLogSentAt
	if from.IsZero() {
		from = now.Add(-24 * time.Hour)
	}

	fromDate := from.Format("2006-01-02")
	toDate   := now.Format("2006-01-02")

	m.mu.Lock()
	devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	// Bridge-Ground own logs
	bgEntries := filterEntriesAfter(logging.GetEntriesForRange(fromDate, toDate), from)
	for _, d := range devices {
		if d.AppName != "Bridge-Ground" || d.DeviceID == "" || d.DeviceToken == "" {
			continue
		}
		if err := m.sendLogBatch(d.DeviceToken, d.DeviceID, "Bridge-Ground", bgEntries); err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Log send failed for Bridge-Ground: %v", err))
		}
		break
	}

	// Per-app logs
	for _, app := range m.apps.List() {
		if app.LogDir == "" || app.LogPrefix == "" {
			continue
		}
		appEntries := filterEntriesAfter(logging.ReadAppLogsFromDir(app.LogDir, app.LogPrefix, fromDate, toDate), from)
		if len(appEntries) == 0 {
			continue
		}
		for _, d := range devices {
			if d.AppName == app.Name && d.Hostname == app.Hostname && d.DeviceID != "" && d.DeviceToken != "" {
				if err := m.sendLogBatch(d.DeviceToken, d.DeviceID, app.Name, appEntries); err != nil {
					logging.Warn("PORTAL", fmt.Sprintf("Log send failed for %s/%s: %v", app.Name, app.Hostname, err))
				}
				break
			}
		}
	}

	m.lastLogSentAt = now
}

func (m *Manager) sendLogBatch(token, deviceID, appName string, entries []logging.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if len(entries) > maxLogBatch {
		entries = entries[len(entries)-maxLogBatch:]
	}
	logEntries := make([]LogEntry, len(entries))
	for i, e := range entries {
		logEntries[i] = LogEntry{Timestamp: e.Timestamp, Level: e.Level, Tag: e.Tag, Message: e.Message}
	}
	return m.client.SendLogs(token, LogsRequest{DeviceID: deviceID, App: appName, Entries: logEntries})
}

// filterEntriesAfter returns only entries whose timestamp is strictly after `after`.
func filterEntriesAfter(entries []logging.LogEntry, after time.Time) []logging.LogEntry {
	if after.IsZero() || len(entries) == 0 {
		return entries
	}
	afterStr := after.Format("2006-01-02 15:04:05.000")
	var result []logging.LogEntry
	for _, e := range entries {
		if e.Timestamp > afterStr {
			result = append(result, e)
		}
	}
	return result
}

// checkAndUploadScreenshots polls the Worker for pending screenshot requests and uploads
// any available screenshots from the local AppRegistry.
func (m *Manager) checkAndUploadScreenshots() {
	m.mu.Lock()
	devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	for _, d := range devices {
		if d.AppName == "Bridge-Ground" || d.DeviceID == "" || d.DeviceToken == "" {
			continue
		}
		pending, err := m.client.CheckScreenshotPending(d.DeviceToken, d.DeviceID)
		if err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Screenshot pending check failed for %s: %v", d.AppName, err))
			continue
		}
		if !pending {
			continue
		}

		app, ok := m.findAppInfo(d.AppName, d.Hostname)
		if !ok {
			continue
		}
		data, _, _, hasCache := m.apps.GetScreenshot(app.ID)
		if !hasCache || len(data) == 0 {
			// No cached screenshot — attempt a live GDI capture (Windows only; no-op on other platforms)
			captured, err := server.CaptureAppScreen(app.Name)
			if err != nil || len(captured) == 0 {
				logging.Warn("PORTAL", fmt.Sprintf("No screenshot available for %s: %v", d.AppName, err))
				continue
			}
			m.apps.StoreScreenshot(app.ID, captured, "image/jpeg")
			data = captured
		}
		if err := m.client.UploadScreenshot(d.DeviceToken, d.DeviceID, data); err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Screenshot upload failed for %s: %v", d.AppName, err))
		} else {
			logging.Info("PORTAL", fmt.Sprintf("Screenshot uploaded for %s (%s)", d.AppName, d.Hostname))
		}
	}
}

// findAppInfo returns the AppInfo whose (Name, Hostname) matches the given pair.
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
