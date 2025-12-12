package config

import (
	"encoding/json"
	"os"
)

// Version is the application version
var Version = "0.0.1"

type Config struct {
	APISettings    APISettings    `json:"apiSettings"`
	SyncSettings   SyncSettings   `json:"syncSettings"`
	ServerSettings ServerSettings `json:"serverSettings"`
}

type APISettings struct {
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type SyncSettings struct {
	SyncIntervalMinutes int  `json:"syncIntervalMinutes"`
	SyncOnStartup       bool `json:"syncOnStartup"`
	AutoSyncEnabled     bool `json:"autoSyncEnabled"`
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
				SyncOnStartup:       true,
				AutoSyncEnabled:     true,
			},
			ServerSettings: ServerSettings{
				Port: 3000,
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
