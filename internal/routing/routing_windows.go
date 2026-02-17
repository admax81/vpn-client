//go:build windows

package routing

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"

	"github.com/user/vpn-client/internal/procutil"
)

// getDefaultGateway retrieves the current default gateway (Windows).
func (m *Manager) getDefaultGateway() (netip.Addr, uint32, error) {
	cmd := procutil.HideWindow(exec.Command("powershell", "-Command",
		"Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Select-Object -First 1 -ExpandProperty NextHop"))
	out, err := cmd.Output()
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("failed to get default gateway: %w", err)
	}

	gwStr := strings.TrimSpace(string(out))
	gw, err := netip.ParseAddr(gwStr)
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("failed to parse gateway: %w", err)
	}

	cmd = procutil.HideWindow(exec.Command("powershell", "-Command",
		"Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Select-Object -First 1 -ExpandProperty InterfaceIndex"))
	out, err = cmd.Output()
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("failed to get interface index: %w", err)
	}

	var ifIdx uint32
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &ifIdx)

	return gw, ifIdx, nil
}

// GetInterfaceIndexByName retrieves the interface index by name (Windows).
func (m *Manager) GetInterfaceIndexByName(name string) (uint32, error) {
	cmd := procutil.HideWindow(exec.Command("powershell", "-Command",
		fmt.Sprintf("Get-NetAdapter -Name '%s' | Select-Object -ExpandProperty InterfaceIndex", name)))
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get interface index for %s: %w", name, err)
	}

	var ifIdx uint32
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &ifIdx)
	if ifIdx == 0 {
		return 0, fmt.Errorf("invalid interface index for %s", name)
	}
	return ifIdx, nil
}

// GetInterfaceIndexByIP retrieves the interface index by IP address (Windows).
func (m *Manager) GetInterfaceIndexByIP(ip string) (uint32, error) {
	cmd := procutil.HideWindow(exec.Command("powershell", "-Command",
		fmt.Sprintf("(Get-NetIPAddress -IPAddress '%s').InterfaceIndex", ip)))
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get interface index for IP %s: %w", ip, err)
	}

	var ifIdx uint32
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &ifIdx)
	if ifIdx == 0 {
		return 0, fmt.Errorf("invalid interface index for IP %s", ip)
	}
	return ifIdx, nil
}

// addSystemRoute adds a route to the Windows routing table.
func (m *Manager) addSystemRoute(route *Route) error {
	dest := route.Destination.Addr().String()
	isIPv6 := route.Destination.Addr().Is6()

	gateway := route.Gateway.String()
	useOnLink := !route.Gateway.IsValid()
	if useOnLink {
		if isIPv6 {
			gateway = "::"
		} else {
			gateway = "0.0.0.0"
		}
	}

	if isIPv6 {
		cmd := procutil.HideWindow(exec.Command("netsh", "interface", "ipv6", "add", "route",
			fmt.Sprintf("%s/%d", dest, route.Destination.Bits()),
			fmt.Sprintf("%d", route.Interface),
			gateway,
			"metric="+fmt.Sprintf("%d", route.Metric),
		))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add IPv6 route: %w: %s", err, string(out))
		}
	} else {
		mask := CIDRMaskString(route.Destination)
		var args []string
		if useOnLink && route.Interface != 0 {
			args = []string{"route", "add", dest, "mask", mask, gateway, "metric", fmt.Sprintf("%d", route.Metric), "if", fmt.Sprintf("%d", route.Interface)}
		} else {
			args = []string{"route", "add", dest, "mask", mask, gateway, "metric", fmt.Sprintf("%d", route.Metric)}
			if route.Interface != 0 {
				args = append(args, "if", fmt.Sprintf("%d", route.Interface))
			}
		}
		cmd := procutil.HideWindow(exec.Command(args[0], args[1:]...))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add IPv4 route: %w: %s", err, string(out))
		}
	}

	return nil
}

// removeSystemRoute removes a route from the Windows routing table.
func (m *Manager) removeSystemRoute(route *Route) error {
	dest := route.Destination.Addr().String()
	isIPv6 := route.Destination.Addr().Is6()

	if isIPv6 {
		cmd := procutil.HideWindow(exec.Command("netsh", "interface", "ipv6", "delete", "route",
			fmt.Sprintf("%s/%d", dest, route.Destination.Bits()),
			fmt.Sprintf("%d", route.Interface),
		))
		cmd.CombinedOutput()
	} else {
		mask := CIDRMaskString(route.Destination)
		cmd := procutil.HideWindow(exec.Command("route", "delete", dest, "mask", mask))
		cmd.CombinedOutput()
	}

	return nil
}

// EnsureVPNServerRoute ensures the VPN server IP is routed via the original gateway (Windows).
func (m *Manager) EnsureVPNServerRoute(serverIP string) error {
	addr, err := netip.ParseAddr(serverIP)
	if err != nil {
		return fmt.Errorf("invalid server IP: %w", err)
	}

	prefix := netip.PrefixFrom(addr, 32)
	dest := prefix.Addr().String()
	mask := "255.255.255.255"

	cmd := procutil.HideWindow(exec.Command("route", "add",
		dest, "mask", mask,
		m.originalGW.String(),
		"metric", "1",
		"if", fmt.Sprintf("%d", m.originalIfIdx),
	))

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add VPN server route: %w: %s", err, string(out))
	}

	return nil
}

// RemoveVPNServerRoute removes the VPN server route (Windows).
func (m *Manager) RemoveVPNServerRoute(serverIP string) error {
	cmd := procutil.HideWindow(exec.Command("route", "delete", serverIP))
	cmd.CombinedOutput()
	return nil
}
