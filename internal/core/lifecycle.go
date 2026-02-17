package core

import (
	"time"

	"github.com/user/vpn-client/internal/logger"
)

// HandlePowerEvent handles system power events.
func (s *Service) HandlePowerEvent(suspend bool) {
	if suspend {
		// System going to sleep
		s.mu.Lock()
		s.wasConnected = (s.state == StateConnected)
		s.mu.Unlock()

		if s.wasConnected {
			// Pause/disconnect gracefully
			s.Disconnect()
		}
	} else {
		// System waking up
		if s.wasConnected {
			// Wait for network to be ready
			time.Sleep(5 * time.Second)
			go func() {
				defer logger.Recover("powerResumeReconnect")
				s.Connect()
			}()
		}
	}
}

// HandleNetworkChange handles network change events.
func (s *Service) HandleNetworkChange() {
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == StateConnected {
		// Reconnect on network change
		go func() {
			defer logger.Recover("handleNetworkChange")
			time.Sleep(2 * time.Second) // Debounce
			if s.tunnel != nil {
				s.tunnel.Reconnect()
			}
		}()
	}
}
