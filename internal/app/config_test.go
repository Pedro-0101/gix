package app

import (
	"testing"

	"gix/internal/config"
)

func TestConfigServiceSaveUpdatesCurrent(t *testing.T) {
	t.Setenv("AppData", t.TempDir())

	s := NewConfigService()
	c := *s.Get()
	c.Opacity = 42

	called := false
	s.OnSave(func(cfg *config.Config) { called = true })

	if err := s.Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if s.Current().Opacity != 42 {
		t.Fatalf("Current não atualizou: %d", s.Current().Opacity)
	}
	if !called {
		t.Fatal("callback OnSave não foi chamado")
	}
}
