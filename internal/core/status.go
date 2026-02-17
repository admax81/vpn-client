package core

// GetStatusPayload returns the current status.
func (s *Service) GetStatusPayload() *StatusPayload {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &StatusPayload{
		State: string(s.state),
	}

	if s.tunnel != nil {
		cfg := s.configManager.Get()
		status.Protocol = string(cfg.Protocol)
		status.ServerAddress = s.tunnel.ServerIP()
		status.LocalIP = s.tunnel.LocalIP().String()

		stats := s.tunnel.Stats()
		status.BytesSent = stats.BytesSent
		status.BytesReceived = stats.BytesReceived
	}

	if !s.connectedAt.IsZero() {
		status.ConnectedAt = s.connectedAt
	}

	if s.lastError != nil {
		status.Error = s.lastError.Error()
	}

	return status
}

// broadcastStatus sends status update to listener.
func (s *Service) broadcastStatus() {
	s.mu.RLock()
	listener := s.statusListener
	s.mu.RUnlock()
	if listener != nil {
		listener(s.GetStatusPayload())
	}
}

// setError sets error state and broadcasts status.
func (s *Service) setError(err error) {
	s.mu.Lock()
	s.state = StateError
	s.lastError = err
	s.mu.Unlock()
	s.broadcastStatus()
}
