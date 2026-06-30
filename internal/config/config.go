package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Version is the application version
var Version = "4.1.0"

type Config struct {
	APISettings    APISettings    `json:"apiSettings"`
	SyncSettings   SyncSettings   `json:"syncSettings"`
	ServerSettings ServerSettings `json:"serverSettings"`
	SystemSettings SystemSettings `json:"systemSettings"`
	PortalSettings PortalSettings `json:"portalSettings"`
}

type SystemSettings struct {
	RunOnStartup bool `json:"runOnStartup"`
	StartHidden  bool `json:"startHidden"`
}

type APISettings struct {
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type SyncSettings struct {
	SyncIntervalMinutes int          `json:"syncIntervalMinutes"`
	SyncOnStartup       bool         `json:"syncOnStartup"`
	AutoSyncEnabled     bool         `json:"autoSyncEnabled"`
	SyncTargets         *SyncTargets `json:"syncTargets,omitempty"`
}

type SyncTargets struct {
	Shops     bool `json:"shops"`
	ShopNews  bool `json:"shopNews"`
	EventNews bool `json:"eventNews"`
	Specials  bool `json:"specials"`
	Sales     bool `json:"sales"`
	ShopApp   bool `json:"shopApp"`
	Genres    bool `json:"genres"`
	Floors    bool `json:"floors"`
}

type ServerSettings struct {
	Port int `json:"port"`
}

// PortalSettings holds Portal CMS integration configuration.
type PortalSettings struct {
	WorkerBaseURL            string         `json:"workerBaseUrl"`
	RegistrationToken        string         `json:"registrationToken"`
	StatusReportIntervalSecs int            `json:"statusReportIntervalSecs"`
	FirebaseDatabaseURL      string         `json:"firebaseDatabaseUrl"`
	Devices                  []PortalDevice `json:"devices"`
}

// PortalDevice stores per-device Portal CMS credentials.
type PortalDevice struct {
	AppName       string   `json:"appName"`
	Hostname      string   `json:"hostname"`
	PendingID     string   `json:"pendingId,omitempty"`
	DeviceID      string   `json:"deviceId,omitempty"`
	DeviceToken   string   `json:"deviceToken,omitempty"`
	SettingsDir   string   `json:"settingsDir,omitempty"`
	SettingsFiles []string `json:"settingsFiles,omitempty"`
}

// configFilePath returns the canonical path for config.json under %APPDATA%\TTI\BridgeGround\.
func configFilePath() string {
	appData, _ := os.UserConfigDir()
	dir := filepath.Join(appData, "TTI", "BridgeGround")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json")
}

func defaultConfig() *Config {
	return &Config{
		APISettings: APISettings{
			BaseURL: "https://api.example.com/api/",
		},
		SyncSettings: SyncSettings{
			SyncIntervalMinutes: 60,
			SyncOnStartup:       false,
			AutoSyncEnabled:     false,
			SyncTargets: &SyncTargets{
				Shops:     false,
				ShopNews:  false,
				EventNews: false,
				Specials:  false,
				Sales:     false,
				ShopApp:   false,
				Genres:    false,
				Floors:    false,
			},
		},
		ServerSettings: ServerSettings{
			Port: 8090,
		},
		SystemSettings: SystemSettings{
			RunOnStartup: false,
		},
		PortalSettings: PortalSettings{
			StatusReportIntervalSecs: 3600,
			Devices:                  []PortalDevice{},
		},
	}
}

// LoadConfig reads config from %APPDATA%\TTI\BridgeGround\config.json.
// Returns defaults if the file does not yet exist.
func LoadConfig() (*Config, error) {
	path := configFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return defaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.SyncSettings.SyncTargets == nil {
		cfg.SyncSettings.SyncTargets = &SyncTargets{}
	}
	if cfg.PortalSettings.Devices == nil {
		cfg.PortalSettings.Devices = []PortalDevice{}
	}
	if cfg.PortalSettings.StatusReportIntervalSecs == 0 {
		cfg.PortalSettings.StatusReportIntervalSecs = 3600
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(), data, 0644)
}
