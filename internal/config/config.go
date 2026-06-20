package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DefaultModel é o modelo padrão do sistema.
const DefaultModel = "google/gemini-2.5-flash-lite"

// Models é a lista fixa de modelos disponíveis para o usuário escolher.
var Models = []string{
	"google/gemini-2.5-flash-lite",
	"google/gemini-2.5-flash",
	"google/gemini-2.5-pro",
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"openai/o3-mini",
	"anthropic/claude-sonnet-4-20250514",
	"anthropic/claude-3.5-haiku",
	"deepseek/deepseek-chat",
	"deepseek/deepseek-r1",
	"meta-llama/llama-3.3-70b-instruct",
	"mistral/mistral-large",
}

type Config struct {
	Theme           string `json:"theme"`
	Language        string `json:"language"`
	OpenKey         string `json:"open_key"`
	OpenIntervalMs  int    `json:"open_interval_ms"`
	CloseKey        string `json:"close_key"`
	CloseIntervalMs int    `json:"close_interval_ms"`
	Model           string `json:"model"`
	APIKey          string `json:"api_key"`
	SystemPrompt    string `json:"system_prompt"`
}

func Default() *Config {
	return &Config{
		Theme:           "light",
		Language:        "pt",
		OpenKey:         "Space",
		OpenIntervalMs:  500,
		CloseKey:        "Escape",
		CloseIntervalMs: 500,
		Model:           DefaultModel,
		APIKey:          "",
		SystemPrompt:    "Responda de forma direta e objetiva.",
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
	if loaded.Model == "" || !isValidModel(loaded.Model) {
		loaded.Model = cfg.Model
	}
	if loaded.SystemPrompt == "" {
		loaded.SystemPrompt = cfg.SystemPrompt
	}
	// APIKey vazio é válido: cai para a variável de ambiente em ResolveAPIKey.
	return &loaded
}

func isValidModel(model string) bool {
	for _, m := range Models {
		if m == model {
			return true
		}
	}
	return false
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

// ResolveAPIKey retorna a chave das settings, ou, se vazia, a variável de
// ambiente OPENROUTER_API_KEY.
func (c *Config) ResolveAPIKey() string {
	if strings.TrimSpace(c.APIKey) != "" {
		return strings.TrimSpace(c.APIKey)
	}
	return os.Getenv("OPENROUTER_API_KEY")
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
