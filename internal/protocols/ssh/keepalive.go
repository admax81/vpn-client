package ssh

import (
	"time"

	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/protocols"
)

// keepalive maintains the SSH connection with periodic keepalive messages.
// It sends an immediate first ping, then repeats every KeepAliveInterval seconds.
// After KeepAliveRetries consecutive failures it triggers a reconnect.
func (t *Tunnel) keepalive() {
	interval := time.Duration(t.cfg.KeepAliveInterval) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}
	maxRetries := t.cfg.KeepAliveRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	logger.Info("SSH keepalive started: interval=%s, retries=%d", interval, maxRetries)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failures := 0

	// sendPing sends a single keepalive and returns true if the connection is alive.
	sendPing := func() bool {
		if t.client == nil {
			return false
		}
		_, _, err := t.client.SendRequest("keepalive@openssh.com", true, nil)
		if err != nil {
			failures++
			logger.Error("SSH keepalive failed (%d/%d): %v", failures, maxRetries, err)
			return false
		}
		if failures > 0 {
			logger.Info("SSH keepalive recovered after %d failures", failures)
		}
		failures = 0
		return true
	}

	// Immediate first ping right after connection
	if !sendPing() && failures >= maxRetries {
		logger.Error("SSH keepalive: connection dead on first check, reconnecting")
		t.SetState(protocols.StateReconnecting, "Connection lost, reconnecting", nil)
		go t.Reconnect()
		return
	}

	for {
		select {
		case <-t.stopCh:
			logger.Debug("SSH keepalive stopped")
			return
		case <-ticker.C:
			if !sendPing() && failures >= maxRetries {
				logger.Error("SSH keepalive: %d consecutive failures, reconnecting", failures)
				t.SetState(protocols.StateReconnecting, "Connection lost, reconnecting", nil)
				go t.Reconnect()
				return
			}
		}
	}
}
