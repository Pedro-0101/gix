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

func TestDefaultNotesSettings(t *testing.T) {
	c := Default()
	if c.NotesLineLimit != 30 {
		t.Errorf("NotesLineLimit default = %d, want 30", c.NotesLineLimit)
	}
	if c.NotesIntegrationMode != "append" {
		t.Errorf("NotesIntegrationMode default = %q, want %q", c.NotesIntegrationMode, "append")
	}
}

func TestLoadFallsBackOnInvalidNotesSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	// Config com valores inválidos para as notas: limite <= 0 e modo desconhecido.
	cfgJSON := `{"notes_line_limit": 0, "notes_integration_mode": "nonsense"}`
	if err := os.WriteFile(filepath.Join(dir, "gix", "config.json"), []byte(cfgJSON), 0o644); err != nil {
		// O diretório pode não existir ainda; cria e tenta de novo.
		_ = os.MkdirAll(filepath.Join(dir, "gix"), 0o755)
		if err := os.WriteFile(filepath.Join(dir, "gix", "config.json"), []byte(cfgJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c := Load()
	if c.NotesLineLimit != 30 {
		t.Errorf("limite inválido deveria cair para 30, veio %d", c.NotesLineLimit)
	}
	if c.NotesIntegrationMode != "append" {
		t.Errorf("modo inválido deveria cair para 'append', veio %q", c.NotesIntegrationMode)
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
