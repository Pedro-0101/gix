package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Theme           string `json:"theme"`
	Language        string `json:"language"`
	OpenKey         string `json:"open_key"`
	OpenIntervalMs  int    `json:"open_interval_ms"`
	CloseKey        string `json:"close_key"`
	CloseIntervalMs int    `json:"close_interval_ms"`
}

func Default() *Config {
	return &Config{
		Theme:           "light",
		Language:        "pt",
		OpenKey:         "Space",
		OpenIntervalMs:  500,
		CloseKey:        "Escape",
		CloseIntervalMs: 500,
	}
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gix"), nil
}

func configPath() (string, error) {
	d, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func Load() *Config {
	cfg := Default()
	path, err := configPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return cfg
	}
	if loaded.Theme == "" {
		loaded.Theme = cfg.Theme
	}
	if loaded.Language == "" {
		loaded.Language = cfg.Language
	}
	if loaded.OpenKey == "" {
		loaded.OpenKey = cfg.OpenKey
	}
	if loaded.OpenIntervalMs <= 0 {
		loaded.OpenIntervalMs = cfg.OpenIntervalMs
	}
	if loaded.CloseKey == "" {
		loaded.CloseKey = cfg.CloseKey
	}
	if loaded.CloseIntervalMs <= 0 {
		loaded.CloseIntervalMs = cfg.CloseIntervalMs
	}
	return &loaded
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
