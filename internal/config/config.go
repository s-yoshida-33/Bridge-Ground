package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Version is the application version
var Version = "4.0.8"

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
	AppName      string   `json:"appName"`
	Hostname     string   `json:"hostname"`
	PendingID    string   `json:"pendingId,omitempty"`
	DeviceID     string   `json:"deviceId,omitempty"`
	DeviceToken  string   `json:"deviceToken,omitempty"`
	SettingsDir  string   `json:"settingsDir,omitempty"`
	SettingsFiles []string `json:"settingsFiles,omitempty"`
}

// configFilePath returns the canonical path for config.json.
// Using the executable's directory ensures LoadConfig and SaveConfig
// always operate on the same file regardless of the working directory.
func configFilePath() string {
	if exePath, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exePath), "config.json")
	}
	return "config.json"
}

// LoadConfig reads the config from the standard location
func LoadConfig() (*Config, error) {
	// Priority: {exeDir}/config.json → config.json (cwd) → default template
	primary := configFilePath()
	paths := []string{primary, "config.json", "src/config/api_config_default.json"}

	// Deduplicate in case exeDir == cwd
	seen := map[string]bool{}
	var unique []string
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			if !seen[abs] {
				seen[abs] = true
				unique = append(unique, p)
			}
		}
	}

	var configPath string
	for _, p := range unique {
		if _, err := os.Stat(p); err == nil {
			configPath = p
			break
		}
	}

	if configPath == "" {
		// Return default if not found
		return &Config{
			APISettings: APISettings{
				BaseURL: "https://api.example.com/api/", // Placeholder
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
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set default sync targets if not present in config
	if cfg.SyncSettings.SyncTargets == nil {
		cfg.SyncSettings.SyncTargets = &SyncTargets{
			Shops:     false,
			ShopNews:  false,
			EventNews: false,
			Specials:  false,
			Sales:     false,
			ShopApp:   false,
			Genres:    false,
			Floors:    false,
		}
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
