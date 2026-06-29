package app

import (
	"sync"

	"gix/internal/config"
)

type ConfigService struct {
	mu     sync.RWMutex
	cfg    *config.Config
	onSave []func(*config.Config)
}

func NewConfigService() *ConfigService {
	return &ConfigService{cfg: config.Load()}
}

func (s *ConfigService) Get() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := *s.cfg
	return &c
}

func (s *ConfigService) Current() *config.Config {
	return s.Get()
}

func (s *ConfigService) OnSave(fn func(*config.Config)) {
	s.mu.Lock()
	s.onSave = append(s.onSave, fn)
	s.mu.Unlock()
}

func (s *ConfigService) Save(c config.Config) error {
	if err := c.Save(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = &c
	cbs := append([]func(*config.Config){}, s.onSave...)
	s.mu.Unlock()
	for _, fn := range cbs {
		cp := c
		fn(&cp)
	}
	return nil
}
