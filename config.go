package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds OAuth credentials.
type Config struct {
	AppKey       string `json:"app_key"`
	AppSecret    string `json:"app_secret"`
	RefreshToken string `json:"refresh_token"`
}

// defaultConfigPath returns ~/.config/dropbox-appender/config.json.
func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dropbox-appender", "config.json")
}

// loadConfig reads config from file, then applies env var overrides.
func loadConfig(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, cfg)
	}

	// Env vars override file values
	if v := os.Getenv("DROPBOX_APP_KEY"); v != "" {
		cfg.AppKey = v
	}
	if v := os.Getenv("DROPBOX_APP_SECRET"); v != "" {
		cfg.AppSecret = v
	}
	if v := os.Getenv("DROPBOX_REFRESH_TOKEN"); v != "" {
		cfg.RefreshToken = v
	}

	return cfg, nil
}

// saveConfig writes config to file, creating directories as needed.
func saveConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
