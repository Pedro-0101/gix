package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	in := []byte("# comentario\nGIX_SERVER_URL=http://localhost:8080\n\nQUOTED=\"com espaco\"\nSEM_IGUAL\nVAZIO=\n")
	got := parseDotEnv(in)

	if got["GIX_SERVER_URL"] != "http://localhost:8080" {
		t.Errorf("GIX_SERVER_URL = %q, want %q", got["GIX_SERVER_URL"], "http://localhost:8080")
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

func TestLoadDotEnvDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GIX_SERVER_URL=http://example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("GIX_SERVER_URL", "ja-definido")

	LoadDotEnv()

	if got := os.Getenv("GIX_SERVER_URL"); got != "ja-definido" {
		t.Errorf("LoadDotEnv sobrescreveu var existente: got %q, want %q", got, "ja-definido")
	}
}

func TestLoadDefaultsServerURL(t *testing.T) {
	t.Setenv("AppData", t.TempDir())
	c := Load()
	if c.ServerURL == "" {
		t.Fatal("ServerURL default vazio")
	}
	if c.Opacity != 85 {
		t.Fatalf("Opacity default = %d, want 85", c.Opacity)
	}
}
