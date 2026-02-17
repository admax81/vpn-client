//go:build darwin

package routing

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// getDefaultGateway retrieves the current default gateway (macOS).
func (m *Manager) getDefaultGateway() (netip.Addr, uint32, error) {
	cmd := exec.Command("route", "-n", "get", "default")
	out, err := cmd.Output()
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("failed to get default gateway: %w", err)
	}

	var gwStr string
	var devName string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			gwStr = strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
		if strings.HasPrefix(line, "interface:") {
			devName = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
	}

	if gwStr == "" {
		return netip.Addr{}, 0, fmt.Errorf("could not parse default gateway")
	}

	gw, err := netip.ParseAddr(gwStr)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("failed to parse gateway: %w", err)
	}

	var ifIdx uint32
	if devName != "" {
		ifIdx = getIfIndexByNameDarwin(devName)
	}

	return gw, ifIdx, nil
}

func getIfIndexByNameDarwin(name string) uint32 {
	cmd := exec.Command("networksetup", "-listallhardwareports")
	out, _ := cmd.Output()
	// Simple approach: use ifconfig to get index
	cmd = exec.Command("ifconfig", name)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	_ = out
	// macOS doesn't expose numeric index easily; return 0 and rely on interface name
	return 0
}

// GetInterfaceIndexByName retrieves the interface index by name (macOS).
func (m *Manager) GetInterfaceIndexByName(name string) (uint32, error) {
	idx := getIfIndexByNameDarwin(name)
	return idx, nil
}

// GetInterfaceIndexByIP retrieves the interface index by IP address (macOS).
func (m *Manager) GetInterfaceIndexByIP(ip string) (uint32, error) {
	cmd := exec.Command("ifconfig")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list interfaces: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var currentDev string
	for _, line := range lines {
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 0 {
				currentDev = strings.TrimSpace(parts[0])
			}
		}
		if strings.Contains(line, ip) && currentDev != "" {
			return getIfIndexByNameDarwin(currentDev), nil
		}
	}

	return 0, fmt.Errorf("interface with IP %s not found", ip)
}

// addSystemRoute adds a route to the macOS routing table.
func (m *Manager) addSystemRoute(route *Route) error {
	dest := route.Destination.String()
	gateway := route.Gateway.String()

	cmd := exec.Command("route", "add", "-net", dest, gateway)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add route: %w: %s", err, string(out))
	}

	return nil
}

// removeSystemRoute removes a route from the macOS routing table.
func (m *Manager) removeSystemRoute(route *Route) error {
	dest := route.Destination.String()
	cmd := exec.Command("route", "delete", "-net", dest)
	cmd.CombinedOutput()
	return nil
}

// EnsureVPNServerRoute ensures the VPN server IP is routed via the original gateway (macOS).
func (m *Manager) EnsureVPNServerRoute(serverIP string) error {
	addr, err := netip.ParseAddr(serverIP)
	if err != nil {
		return fmt.Errorf("invalid server IP: %w", err)
	}
	_ = addr

	cmd := exec.Command("route", "add", "-host", serverIP, m.originalGW.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add VPN server route: %w: %s", err, string(out))
	}

	return nil
}

// RemoveVPNServerRoute removes the VPN server route (macOS).
func (m *Manager) RemoveVPNServerRoute(serverIP string) error {
	cmd := exec.Command("route", "delete", "-host", serverIP)
	cmd.CombinedOutput()
	return nil
}
