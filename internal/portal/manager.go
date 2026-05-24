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

// Manager handles Portal CMS device registration and periodic status reporting.
type Manager struct {
	mu         sync.Mutex
	cfg        *config.Config
	apps       *server.AppRegistry
	saveConfig func(*config.Config) error
	client     *Client
	startTime  time.Time
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
		m.pollPendingApprovals()
		m.reportAllStatus()
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

// pollPendingApprovals checks whether any pending registrations have been approved
// by the Portal CMS admin and promotes them to full device credentials.
func (m *Manager) pollPendingApprovals() {
	m.mu.Lock()
	var pending []config.PortalDevice
	for _, d := range m.cfg.PortalSettings.Devices {
		if d.PendingID != "" && d.DeviceID == "" {
			pending = append(pending, d)
		}
	}
	m.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	token := m.cfg.PortalSettings.RegistrationToken
	changed := false
	for _, d := range pending {
		resp, err := m.client.CheckPending(token, d.PendingID)
		if err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Failed to check pending approval for %s/%s: %v", d.AppName, d.Hostname, err))
			continue
		}
		if resp.Status != "approved" {
			continue
		}
		m.mu.Lock()
		m.upsertDevice(config.PortalDevice{
			AppName:     d.AppName,
			Hostname:    d.Hostname,
			DeviceID:    resp.DeviceID,
			DeviceToken: resp.DeviceToken,
		})
		m.mu.Unlock()
		logging.Info("PORTAL", fmt.Sprintf("Device approved: %s/%s -> deviceId=%s", d.AppName, d.Hostname, resp.DeviceID))
		changed = true
	}

	if changed {
		if err := m.saveConfig(m.cfg); err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Failed to save config after approval: %v", err))
		}
	}
}

func (m *Manager) reportAllStatus() {
	metrics   := CollectMetrics()
	uptimeHrs := int(time.Since(m.startTime).Hours())
	appList   := m.apps.List()

	m.mu.Lock()
	devices := make([]config.PortalDevice, len(m.cfg.PortalSettings.Devices))
	copy(devices, m.cfg.PortalSettings.Devices)
	m.mu.Unlock()

	for _, d := range devices {
		if d.DeviceID == "" || d.DeviceToken == "" {
			continue
		}
		req := m.buildStatusRequest(d, metrics, uptimeHrs, appList)
		if err := m.client.ReportStatus(d.DeviceToken, req); err != nil {
			logging.Warn("PORTAL", fmt.Sprintf("Status report failed for %s/%s: %v", d.AppName, d.Hostname, err))
		}
	}
}

func (m *Manager) buildStatusRequest(d config.PortalDevice, metrics Metrics, uptimeHrs int, apps []server.AppInfo) StatusRequest {
	if d.AppName == "Bridge-Ground" {
		return StatusRequest{
			DeviceID:    d.DeviceID,
			Status:      "online",
			CPU:         metrics.CPU,
			Memory:      metrics.Memory,
			Temperature: metrics.Temperature,
			Storage:     metrics.Storage,
			Uptime:      uptimeHrs,
		}
	}
	status := "offline"
	for _, app := range apps {
		if app.Name == d.AppName && app.Hostname == d.Hostname {
			if app.Online {
				status = "online"
			}
			break
		}
	}
	return StatusRequest{
		DeviceID: d.DeviceID,
		Status:   status,
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

// localIP returns this machine's preferred outbound IP address.
func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "0.0.0.0"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
