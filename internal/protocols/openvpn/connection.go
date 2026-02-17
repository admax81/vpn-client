package openvpn

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/user/vpn-client/internal/procutil"
	"github.com/user/vpn-client/internal/protocols"
)

// Start establishes the OpenVPN tunnel.
func (t *Tunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State() == protocols.StateConnected {
		return fmt.Errorf("tunnel already connected")
	}

	t.SetState(protocols.StateConnecting, "Initializing OpenVPN tunnel", nil)
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Find OpenVPN binary
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	openVPNPath := filepath.Join(filepath.Dir(exePath), openVPNBinary)
	if _, err := os.Stat(openVPNPath); os.IsNotExist(err) {
		// Try PATH
		openVPNPath, err = exec.LookPath("openvpn")
		if err != nil {
			t.SetState(protocols.StateError, "OpenVPN not found", err)
			return fmt.Errorf("openvpn binary not found")
		}
	}

	// Resolve server IP from config file if possible
	// For now just use placeholder - will be updated when connected
	t.ServerIPAddr = ""

	// Config path is required
	configPath := t.cfg.ConfigPath
	if configPath == "" {
		t.SetState(protocols.StateError, "OpenVPN config path required", nil)
		return fmt.Errorf("openvpn config path is required")
	}

	// Build command arguments
	args := []string{
		"--config", configPath,
		"--management", "127.0.0.1", strconv.Itoa(managementPort),
		"--management-query-passwords",
		"--management-hold",
		"--dev", t.ifaceCfg.Name,
		"--dev-type", "tun",
	}

	// Start OpenVPN process
	t.process = exec.CommandContext(t.ctx, openVPNPath, args...)
	procutil.HideWindow(t.process)
	t.process.Stdout = os.Stdout
	t.process.Stderr = os.Stderr

	if err := t.process.Start(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to start OpenVPN", err)
		return fmt.Errorf("failed to start openvpn: %w", err)
	}

	// Connect to management interface
	time.Sleep(500 * time.Millisecond) // Give OpenVPN time to start

	if err := t.connectManagement(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to connect to management", err)
		return fmt.Errorf("failed to connect to management: %w", err)
	}

	// Release the hold
	if err := t.sendCommand("hold release"); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to release hold", err)
		return fmt.Errorf("failed to release hold: %w", err)
	}

	// Start management monitor
	go t.monitorManagement()

	// Wait for connection with timeout
	select {
	case <-t.connected:
		t.SetState(protocols.StateConnected, "OpenVPN tunnel established", nil)
	case <-time.After(60 * time.Second):
		t.cleanup()
		t.SetState(protocols.StateError, "Connection timeout", nil)
		return fmt.Errorf("connection timeout")
	case <-t.ctx.Done():
		t.cleanup()
		return t.ctx.Err()
	}

	return nil
}

// Stop terminates the OpenVPN tunnel.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State() == protocols.StateDisconnected {
		return nil
	}

	t.SetState(protocols.StateDisconnecting, "Stopping OpenVPN tunnel", nil)

	if t.cancel != nil {
		t.cancel()
	}

	// Send signal to disconnect
	if t.mgmtConn != nil {
		t.sendCommand("signal SIGTERM")
	}

	t.cleanup()

	t.SetState(protocols.StateDisconnected, "OpenVPN tunnel stopped", nil)
	t.Close() // Close stateChanges channel so monitorTunnel exits

	return nil
}

// Reconnect attempts to reconnect the tunnel.
func (t *Tunnel) Reconnect() error {
	t.SetState(protocols.StateReconnecting, "Reconnecting", nil)

	// Send reconnect command via management
	if t.mgmtConn != nil {
		if err := t.sendCommand("signal SIGHUP"); err != nil {
			// Fallback to full restart
			if err := t.Stop(); err != nil {
				return err
			}
			t.ResetChannel()
			return t.Start(context.Background())
		}
		return nil
	}

	if err := t.Stop(); err != nil {
		return err
	}
	t.ResetChannel()
	return t.Start(context.Background())
}
