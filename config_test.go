package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"app_key":"key1","app_secret":"secret1","refresh_token":"refresh1"}`), 0600)

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "key1" || cfg.AppSecret != "secret1" || cfg.RefreshToken != "refresh1" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, err := loadConfig("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "" || cfg.AppSecret != "" || cfg.RefreshToken != "" {
		t.Errorf("expected empty config, got: %+v", cfg)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"app_key":"file_key","app_secret":"file_secret","refresh_token":"file_refresh"}`), 0600)

	t.Setenv("DROPBOX_APP_KEY", "env_key")
	t.Setenv("DROPBOX_APP_SECRET", "env_secret")
	t.Setenv("DROPBOX_REFRESH_TOKEN", "env_refresh")

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "env_key" || cfg.AppSecret != "env_secret" || cfg.RefreshToken != "env_refresh" {
		t.Errorf("env vars should override file, got: %+v", cfg)
	}
}

func TestLoadConfig_PartialEnvOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"app_key":"file_key","app_secret":"file_secret","refresh_token":"file_refresh"}`), 0600)

	t.Setenv("DROPBOX_APP_KEY", "env_key")

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "env_key" {
		t.Errorf("expected env_key, got %s", cfg.AppKey)
	}
	if cfg.AppSecret != "file_secret" {
		t.Errorf("expected file_secret, got %s", cfg.AppSecret)
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "subdir", "config.json")

	cfg := &Config{AppKey: "k", AppSecret: "s", RefreshToken: "r"}
	err := saveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if loaded.AppKey != "k" || loaded.AppSecret != "s" || loaded.RefreshToken != "r" {
		t.Errorf("round-trip failed: %+v", loaded)
	}
}
