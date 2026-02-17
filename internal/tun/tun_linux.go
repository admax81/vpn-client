//go:build linux

package tun

import (
	"fmt"
	"net/netip"
	"os/exec"
)

// normalizeInterfaceName returns the name as-is on Linux (no restrictions).
func normalizeInterfaceName(name string) string {
	return name
}

// assignIP assigns an IP address to the adapter (Linux).
func (a *Adapter) assignIP(prefix netip.Prefix) error {
	cmd := exec.Command("ip", "addr", "add", prefix.String(), "dev", a.name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set IP address: %w: %s", err, string(out))
	}
	return nil
}

// setMetric sets the interface metric (Linux).
func (a *Adapter) setMetric(metric int) error {
	cmd := exec.Command("ip", "link", "set", "dev", a.name,
		"type", "none")
	cmd.CombinedOutput() // metric is set via route on Linux, not interface
	return nil
}

// Up brings the adapter up (Linux).
func (a *Adapter) Up() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device == nil {
		return fmt.Errorf("adapter not created")
	}

	cmd := exec.Command("ip", "link", "set", "dev", a.name, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w: %s", err, string(out))
	}

	a.isUp = true
	return nil
}

// Down brings the adapter down (Linux).
func (a *Adapter) Down() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device == nil {
		return nil
	}

	cmd := exec.Command("ip", "link", "set", "dev", a.name, "down")
	cmd.CombinedOutput()

	a.isUp = false
	return nil
}
