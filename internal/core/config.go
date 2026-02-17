package core

import (
	"net/netip"

	"github.com/user/vpn-client/internal/config"
)

// GetState returns the current state.
func (s *Service) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() *config.Config {
	return s.configManager.Get()
}

// UpdateConfig updates the configuration.
func (s *Service) UpdateConfig(cfg *config.Config) error {
	return s.configManager.Update(cfg)
}

// ReloadConfig re-reads the configuration file from disk.
func (s *Service) ReloadConfig() error {
	return s.configManager.Load()
}

// GetLocalIP returns the tunnel local IP.
func (s *Service) GetLocalIP() netip.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.tunnel != nil {
		return s.tunnel.LocalIP()
	}
	return netip.Addr{}
}
