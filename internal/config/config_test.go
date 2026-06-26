package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	in := []byte("# comentario\nOPENROUTER_API_KEY=abc123\n\nQUOTED=\"com espaco\"\nSEM_IGUAL\nVAZIO=\n")
	got := parseDotEnv(in)

	if got["OPENROUTER_API_KEY"] != "abc123" {
		t.Errorf("OPENROUTER_API_KEY = %q, want %q", got["OPENROUTER_API_KEY"], "abc123")
	}
	if got["QUOTED"] != "com espaco" {
		t.Errorf("QUOTED = %q, want %q", got["QUOTED"], "com espaco")
	}
	if got["VAZIO"] != "" {
		t.Errorf("VAZIO = %q, want empty", got["VAZIO"])
	}
	if _, ok := got["VAZIO"]; !ok {
		t.Errorf("VAZIO deveria existir como chave com valor vazio")
	}
	if _, ok := got["SEM_IGUAL"]; ok {
		t.Errorf("linha sem '=' nao deveria virar chave")
	}
	if _, ok := got["# comentario"]; ok {
		t.Errorf("comentario nao deveria virar chave")
	}
}

func TestResolveAPIKeyPrefersConfig(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "do-ambiente")
	c := &Config{APIKey: "das-settings"}
	if got := c.ResolveAPIKey(); got != "das-settings" {
		t.Errorf("ResolveAPIKey() = %q, want %q", got, "das-settings")
	}
}

func TestResolveAPIKeyFallsBackToEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "do-ambiente")
	c := &Config{APIKey: ""}
	if got := c.ResolveAPIKey(); got != "do-ambiente" {
		t.Errorf("ResolveAPIKey() = %q, want %q", got, "do-ambiente")
	}
}

func TestLoadDotEnvDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OPENROUTER_API_KEY=do-arquivo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("OPENROUTER_API_KEY", "ja-definido")

	LoadDotEnv()

	if got := os.Getenv("OPENROUTER_API_KEY"); got != "ja-definido" {
		t.Errorf("LoadDotEnv sobrescreveu var existente: got %q, want %q", got, "ja-definido")
	}
}
