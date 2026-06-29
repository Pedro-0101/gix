package app

import "gix/internal/secret"

// TokenService expõe o cofre de sessão (DPAPI no Windows) ao frontend, para o
// par de tokens JWT não viver em localStorage no WebView. O frontend serializa
// {access, refresh} num JSON opaco; o Go só cifra/decifra/persiste a string.
type TokenService struct {
	store *secret.Store
}

func NewTokenService() *TokenService {
	return &TokenService{store: secret.New()}
}

// Load devolve o blob salvo, ou "" se não houver sessão persistida.
func (s *TokenService) Load() (string, error) { return s.store.Load() }

// Save cifra e persiste o blob; string vazia limpa o cofre.
func (s *TokenService) Save(data string) error { return s.store.Save(data) }

// Clear apaga a sessão persistida (logout).
func (s *TokenService) Clear() error { return s.store.Clear() }
