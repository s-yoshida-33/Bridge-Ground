package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Version is the application version
var Version = "2.3.0"

type Config struct {
	APISettings    APISettings    `json:"apiSettings"`
	SyncSettings   SyncSettings   `json:"syncSettings"`
	ServerSettings ServerSettings `json:"serverSettings"`
	SystemSettings SystemSettings `json:"systemSettings"`
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
	Genres    bool `json:"genres"`
}

type ServerSettings struct {
	Port int `json:"port"`
}

// LoadConfig reads the config from the standard location
// For this example, we look in the current directory or src/config
func LoadConfig() (*Config, error) {
	// Trying to find the existing config file
	paths := []string{
		"config.json",
		"src/config/api_config_default.json",
	}

	// Add executable directory to search paths
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		// Check config.json in the same directory as the executable
		paths = append([]string{filepath.Join(exeDir, "config.json")}, paths...)
	}

	var configPath string
	for _, p := range paths {
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
					Genres:    false,
				},
			},
			ServerSettings: ServerSettings{
				Port: 8090,
			},
			SystemSettings: SystemSettings{
				RunOnStartup: false,
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
			Genres:    false,
		}
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Always save to config.json in current directory for persistence
	return os.WriteFile("config.json", data, 0644)
}
