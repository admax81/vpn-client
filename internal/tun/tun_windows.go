//go:build windows

package tun

import (
	"fmt"
	"net"
	"net/netip"
	"os/exec"

	"github.com/user/vpn-client/internal/procutil"
)

// normalizeInterfaceName returns the name as-is on Windows (no restrictions).
func normalizeInterfaceName(name string) string {
	return name
}

// assignIP assigns an IP address to the adapter (Windows).
func (a *Adapter) assignIP(prefix netip.Prefix) error {
	mask := net.CIDRMask(prefix.Bits(), 32)
	maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])

	cmd := procutil.HideWindow(exec.Command("netsh", "interface", "ipv4", "set", "address",
		fmt.Sprintf("name=%s", a.name),
		"source=static",
		fmt.Sprintf("address=%s", prefix.Addr().String()),
		fmt.Sprintf("mask=%s", maskStr),
		"gateway=none",
	))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set IP address: %w: %s", err, string(out))
	}
	return nil
}

// setMetric sets the interface metric (Windows).
func (a *Adapter) setMetric(metric int) error {
	cmd := procutil.HideWindow(exec.Command("netsh", "interface", "ipv4", "set", "interface",
		a.name,
		fmt.Sprintf("metric=%d", metric),
	))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set metric: %w: %s", err, string(out))
	}
	return nil
}

// Up brings the adapter up (Windows).
func (a *Adapter) Up() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device == nil {
		return fmt.Errorf("adapter not created")
	}

	cmd := procutil.HideWindow(exec.Command("netsh", "interface", "set", "interface",
		a.name, "admin=enable"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w: %s", err, string(out))
	}

	a.isUp = true
	return nil
}

// Down brings the adapter down (Windows).
func (a *Adapter) Down() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device == nil {
		return nil
	}

	cmd := procutil.HideWindow(exec.Command("netsh", "interface", "set", "interface",
		a.name, "admin=disable"))
	cmd.CombinedOutput()

	a.isUp = false
	return nil
}
