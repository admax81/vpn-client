//go:build linux

package routing

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// getDefaultGateway retrieves the current default gateway (Linux).
func (m *Manager) getDefaultGateway() (netip.Addr, uint32, error) {
	cmd := exec.Command("ip", "route", "show", "default")
	out, err := cmd.Output()
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("failed to get default gateway: %w", err)
	}

	// Parse: "default via 192.168.1.1 dev eth0 ..."
	fields := strings.Fields(strings.TrimSpace(string(out)))
	var gwStr string
	var devName string
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			gwStr = fields[i+1]
		}
		if f == "dev" && i+1 < len(fields) {
			devName = fields[i+1]
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
		ifIdx = getIfIndexByName(devName)
	}

	return gw, ifIdx, nil
}

func getIfIndexByName(name string) uint32 {
	cmd := exec.Command("cat", fmt.Sprintf("/sys/class/net/%s/ifindex", name))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var idx uint32
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &idx)
	return idx
}

// GetInterfaceIndexByName retrieves the interface index by name (Linux).
func (m *Manager) GetInterfaceIndexByName(name string) (uint32, error) {
	idx := getIfIndexByName(name)
	if idx == 0 {
		return 0, fmt.Errorf("interface %s not found", name)
	}
	return idx, nil
}

// GetInterfaceIndexByIP retrieves the interface index by IP address (Linux).
func (m *Manager) GetInterfaceIndexByIP(ip string) (uint32, error) {
	cmd := exec.Command("ip", "-o", "addr", "show")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, ip) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				devName := fields[1]
				return getIfIndexByName(devName), nil
			}
		}
	}

	return 0, fmt.Errorf("interface with IP %s not found", ip)
}

// addSystemRoute adds a route to the Linux routing table.
func (m *Manager) addSystemRoute(route *Route) error {
	dest := route.Destination.String()
	gateway := route.Gateway.String()

	args := []string{"route", "add", dest, "via", gateway}
	if route.Metric > 0 {
		args = append(args, "metric", fmt.Sprintf("%d", route.Metric))
	}

	cmd := exec.Command("ip", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add route: %w: %s", err, string(out))
	}

	return nil
}

// removeSystemRoute removes a route from the Linux routing table.
func (m *Manager) removeSystemRoute(route *Route) error {
	dest := route.Destination.String()
	cmd := exec.Command("ip", "route", "delete", dest)
	cmd.CombinedOutput()
	return nil
}

// EnsureVPNServerRoute ensures the VPN server IP is routed via the original gateway (Linux).
func (m *Manager) EnsureVPNServerRoute(serverIP string) error {
	addr, err := netip.ParseAddr(serverIP)
	if err != nil {
		return fmt.Errorf("invalid server IP: %w", err)
	}

	prefix := netip.PrefixFrom(addr, 32)
	cmd := exec.Command("ip", "route", "add",
		prefix.String(), "via", m.originalGW.String(),
		"metric", "1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add VPN server route: %w: %s", err, string(out))
	}

	return nil
}

// RemoveVPNServerRoute removes the VPN server route (Linux).
func (m *Manager) RemoveVPNServerRoute(serverIP string) error {
	cmd := exec.Command("ip", "route", "delete", serverIP+"/32")
	cmd.CombinedOutput()
	return nil
}
