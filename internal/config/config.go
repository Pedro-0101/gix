package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ModelPricing representa o custo por 1M de tokens (USD).
type ModelPricing struct {
	InputPrice  float64
	OutputPrice float64
}

// CalculateCost calcula o custo em dólares a partir do uso de tokens.
func (p ModelPricing) CalculateCost(promptTokens, completionTokens int) float64 {
	inputCost := p.InputPrice * float64(promptTokens) / 1_000_000
	outputCost := p.OutputPrice * float64(completionTokens) / 1_000_000
	return inputCost + outputCost
}

// ModelPrices mapeia cada modelo ao seu custo por 1M tokens (USD) no OpenRouter.
var ModelPrices = map[string]ModelPricing{
	"google/gemini-2.5-flash-lite":       {0.075, 0.30},
	"google/gemini-2.5-flash":            {0.10, 0.40},
	"google/gemini-2.5-pro":              {1.25, 5.00},
	"openai/gpt-4o":                      {2.50, 10.00},
	"openai/gpt-4o-mini":                 {0.15, 0.60},
	"openai/o3-mini":                     {1.10, 4.40},
	"anthropic/claude-sonnet-4-20250514": {3.00, 15.00},
	"anthropic/claude-3.5-haiku":         {0.80, 4.00},
	"deepseek/deepseek-chat":             {0.27, 1.10},
	"deepseek/deepseek-r1":               {0.55, 2.19},
	"meta-llama/llama-3.3-70b-instruct":  {0.25, 0.25},
	"mistral/mistral-large":              {2.00, 6.00},
}

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
	// Opacity is the background opacity of the palette shell, 0–100 (percent).
	// Higher is more opaque; the remainder lets the acrylic backdrop show through.
	Opacity int `json:"opacity"`
}

func Default() *Config {
	return &Config{
		Theme:                "light",
		Language:             "pt",
		OpenKey:              "Space",
		OpenIntervalMs:       500,
		CloseKey:             "Escape",
		CloseIntervalMs:      500,
		Model:                DefaultModel,
		APIKey:               "",
		SystemPrompt:         "Responda de forma direta e objetiva. Se perceber que o usuário quer registrar ou que seria útil registrar uma informação importante (ideia, aprendizado, decisão etc., chame a ferramenta create_note com título, conteúdo em Markdown e tags — o sistema vai pedir confirmação antes de salvar.",
		Opacity:              85,
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
	if loaded.Opacity <= 0 {
		loaded.Opacity = cfg.Opacity
	}
	if loaded.Opacity > 100 {
		loaded.Opacity = 100
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
