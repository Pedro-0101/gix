// Package secret persiste segredos pequenos (o par de tokens de sessão) cifrados
// em repouso, para não viverem em localStorage no WebView. No Windows usa DPAPI
// (CryptProtectData, escopo do usuário logado); em outras plataformas grava em
// texto por enquanto (keychain/secret-service ficam p/ uma fase futura).
package secret

import (
	"os"
	"path/filepath"
)

type Store struct{ path string }

// New aponta o cofre para <UserConfigDir>/gix/session.bin.
func New() *Store {
	dir, _ := os.UserConfigDir()
	return &Store{path: filepath.Join(dir, "gix", "session.bin")}
}

// Load devolve o segredo decifrado, ou "" se não houver nada salvo.
func (s *Store) Load() (string, error) {
	enc, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if len(enc) == 0 {
		return "", nil
	}
	plain, err := unprotect(enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Save cifra e grava o segredo (modo 0600). String vazia limpa o cofre.
func (s *Store) Save(secret string) error {
	if secret == "" {
		return s.Clear()
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	enc, err := protect([]byte(secret))
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, enc, 0o600)
}

// Clear apaga o blob persistido (logout). Ausência não é erro.
func (s *Store) Clear() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
