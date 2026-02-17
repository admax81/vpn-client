//go:build darwin

package tun

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// assignIP assigns an IP address to the adapter (macOS).
func (a *Adapter) assignIP(prefix netip.Prefix) error {
	addr := prefix.Addr().String()
	// For utun on macOS, use ifconfig with a point-to-point address
	// Calculate a peer address (first IP in subnet) for the p2p link
	masked := prefix.Masked().Addr()
	var peer string
	if masked.Is4() {
		b := masked.As4()
		b[3] = 1
		peer = netip.AddrFrom4(b).String()
	} else {
		peer = addr
	}

	cmd := exec.Command("ifconfig", a.name, "inet", addr, peer, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set IP address: %w: %s", err, string(out))
	}

	// Add subnet route
	bits := prefix.Bits()
	mask := prefixToMask(bits)
	network := prefix.Masked().Addr().String()
	cmd = exec.Command("route", "add", "-net", network+"/"+fmt.Sprintf("%d", bits), "-interface", a.name)
	cmd.CombinedOutput() // Non-fatal

	_ = mask
	return nil
}

// setMetric sets the interface metric (macOS — not directly supported, use route metric).
func (a *Adapter) setMetric(metric int) error {
	// macOS doesn't have interface-level metrics; handled at route level
	return nil
}

// Up brings the adapter up (macOS).
func (a *Adapter) Up() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device == nil {
		return fmt.Errorf("adapter not created")
	}

	cmd := exec.Command("ifconfig", a.name, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w: %s", err, string(out))
	}

	a.isUp = true
	return nil
}

// Down brings the adapter down (macOS).
func (a *Adapter) Down() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device == nil {
		return nil
	}

	cmd := exec.Command("ifconfig", a.name, "down")
	cmd.CombinedOutput()

	a.isUp = false
	return nil
}

func prefixToMask(bits int) string {
	mask := make([]byte, 4)
	for i := 0; i < bits; i++ {
		mask[i/8] |= 1 << (7 - i%8)
	}
	parts := make([]string, 4)
	for i, b := range mask {
		parts[i] = fmt.Sprintf("%d", b)
	}
	return strings.Join(parts, ".")
}
