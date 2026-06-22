package app

import (
	"testing"

	"gix/internal/config"
)

func TestConfigServiceSaveUpdatesCurrent(t *testing.T) {
	t.Setenv("AppData", t.TempDir())

	s := NewConfigService()
	c := *s.Get()
	c.SystemPrompt = "novo prompt"

	called := false
	s.OnSave(func(cfg *config.Config) { called = true })

	if err := s.Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if s.Current().SystemPrompt != "novo prompt" {
		t.Fatalf("Current não atualizou: %q", s.Current().SystemPrompt)
	}
	if !called {
		t.Fatal("callback OnSave não foi chamado")
	}
}

func TestConfigServiceModels(t *testing.T) {
	t.Setenv("AppData", t.TempDir())

	s := NewConfigService()
	if len(s.Models()) == 0 {
		t.Fatal("Models vazio")
	}
}
