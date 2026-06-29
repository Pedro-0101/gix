// Package config carrega a configuração LOCAL do desktop gix: preferências de
// shell que só fazem sentido no cliente (tema, opacidade, hotkeys, idioma).
//
// Toda a lógica de backend (IA, modelos, pricing, prefs de usuário como
// system_prompt/note_char_limit/api_key) vive no gix-server e é acessada via
// HTTP. Este config não contém mais nenhum segredo nem campo de IA.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config é o conjunto de preferências locais do desktop gix.
type Config struct {
	Theme           string `json:"theme"`
	Language        string `json:"language"`
	OpenKey         string `json:"open_key"`
	OpenIntervalMs  int    `json:"open_interval_ms"`
	OpenPressCount  int    `json:"open_press_count"`
	CloseKey        string `json:"close_key"`
	CloseIntervalMs int    `json:"close_interval_ms"`
	ClosePressCount int    `json:"close_press_count"`
	// Opacity is the background opacity of the palette shell, 0–100 (percent).
	// Higher is more opaque; the remainder lets the acrylic backdrop show through.
	Opacity int `json:"opacity"`
	// ServerURL é a base do gix-server (ex.: http://localhost:3000 ou
	// https://gix.up.railway.app). O frontend usa para todas as chamadas HTTP.
	ServerURL string `json:"server_url"`
}

func Default() *Config {
	return &Config{
		Theme:           "light",
		Language:        "pt",
		OpenKey:         "Space",
		OpenIntervalMs:  500,
		OpenPressCount:  3,
		CloseKey:        "Escape",
		CloseIntervalMs: 500,
		ClosePressCount: 2,
		Opacity:         85,
		ServerURL:       "http://localhost:3000",
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
	if loaded.OpenPressCount < 2 || loaded.OpenPressCount > 3 {
		loaded.OpenPressCount = cfg.OpenPressCount
	}
	if loaded.CloseKey == "" {
		loaded.CloseKey = cfg.CloseKey
	}
	if loaded.CloseIntervalMs <= 0 {
		loaded.CloseIntervalMs = cfg.CloseIntervalMs
	}
	if loaded.ClosePressCount < 2 || loaded.ClosePressCount > 3 {
		loaded.ClosePressCount = cfg.ClosePressCount
	}
	if loaded.Opacity <= 0 {
		loaded.Opacity = cfg.Opacity
	}
	if loaded.Opacity > 100 {
		loaded.Opacity = 100
	}
	if strings.TrimSpace(loaded.ServerURL) == "" {
		loaded.ServerURL = cfg.ServerURL
	}
	return &loaded
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// parseDotEnv lê linhas CHAVE=VALOR, ignorando comentários (#), linhas em
// branco e linhas sem '='. Remove aspas simples/duplas ao redor do valor.
func parseDotEnv(data []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key == "" {
			continue
		}
		val := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
		out[key] = val
	}
	return out
}

// LoadDotEnv carrega um arquivo .env (no diretório atual e ao lado do
// executável) para variáveis de ambiente ainda não definidas.
func LoadDotEnv() {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for k, v := range parseDotEnv(data) {
			if _, ok := os.LookupEnv(k); !ok {
				os.Setenv(k, v)
			}
		}
	}
}
