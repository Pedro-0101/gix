package secret

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTempStore(t *testing.T) *Store {
	t.Helper()
	return &Store{path: filepath.Join(t.TempDir(), "session.bin")}
}

func TestStoreRoundTrip(t *testing.T) {
	s := newTempStore(t)

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load vazio: %v", err)
	}
	if got != "" {
		t.Fatalf("cofre novo: esperava \"\", veio %q", got)
	}

	const blob = `{"a":"access-jwt","r":"refresh-opaque"}`
	if err := s.Save(blob); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err = s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != blob {
		t.Fatalf("round-trip: esperava %q, veio %q", blob, got)
	}

	// Save("") limpa o cofre (logout).
	if err := s.Save(""); err != nil {
		t.Fatalf("Save vazio: %v", err)
	}
	if got, _ := s.Load(); got != "" {
		t.Fatalf("após limpar: esperava \"\", veio %q", got)
	}
}

func TestClearIsIdempotent(t *testing.T) {
	s := newTempStore(t)
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear sem arquivo deveria ser no-op: %v", err)
	}
}

// No Windows o DPAPI cifra em repouso: o texto puro não pode aparecer no arquivo.
func TestEncryptsAtRestOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI só no Windows; outras plataformas gravam em texto por ora")
	}
	s := newTempStore(t)
	const secret = "refresh-opaque-supersegredo"
	if err := s.Save(secret); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatal("texto puro encontrado no arquivo — DPAPI não cifrou")
	}
}
