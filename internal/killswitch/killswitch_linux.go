//go:build linux

package killswitch

import (
	"fmt"
	"os/exec"
)

const chainName = "VPN_KILLSWITCH"

// Enable activates the kill switch (Linux — iptables).
func (k *KillSwitch) Enable(cfg *Config) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.rulesCreated {
		k.disableUnsafe()
	}

	k.allowLAN = cfg.AllowLAN
	k.vpnServerIP = cfg.VPNServerIP
	k.vpnInterface = cfg.VPNInterface
	k.allowedProcs = cfg.AllowedProcesses

	// Create custom chain
	exec.Command("iptables", "-N", chainName).CombinedOutput()
	exec.Command("iptables", "-F", chainName).CombinedOutput()

	// Allow loopback
	exec.Command("iptables", "-A", chainName, "-o", "lo", "-j", "ACCEPT").CombinedOutput()

	// Allow DHCP
	exec.Command("iptables", "-A", chainName, "-p", "udp", "--sport", "68", "--dport", "67", "-j", "ACCEPT").CombinedOutput()

	// Allow VPN server
	if cfg.VPNServerIP != "" {
		exec.Command("iptables", "-A", chainName, "-d", cfg.VPNServerIP, "-j", "ACCEPT").CombinedOutput()
	}

	// Allow VPN interface
	if cfg.VPNInterface != "" {
		exec.Command("iptables", "-A", chainName, "-o", cfg.VPNInterface, "-j", "ACCEPT").CombinedOutput()
	}

	// Allow LAN
	if cfg.AllowLAN {
		for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"} {
			exec.Command("iptables", "-A", chainName, "-d", cidr, "-j", "ACCEPT").CombinedOutput()
		}
	}

	// Block everything else
	exec.Command("iptables", "-A", chainName, "-j", "DROP").CombinedOutput()

	// Insert chain into OUTPUT
	exec.Command("iptables", "-I", "OUTPUT", "1", "-j", chainName).CombinedOutput()

	k.enabled = true
	k.rulesCreated = true
	return nil
}

// Disable deactivates the kill switch (Linux).
func (k *KillSwitch) Disable() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.disableUnsafe()
}

func (k *KillSwitch) disableUnsafe() error {
	exec.Command("iptables", "-D", "OUTPUT", "-j", chainName).CombinedOutput()
	exec.Command("iptables", "-F", chainName).CombinedOutput()
	exec.Command("iptables", "-X", chainName).CombinedOutput()

	k.enabled = false
	k.rulesCreated = false
	return nil
}

// UpdateVPNInterface updates the allowed VPN interface (Linux).
func (k *KillSwitch) UpdateVPNInterface(interfaceName string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.enabled {
		return nil
	}

	// Remove old interface rule and add new one
	if k.vpnInterface != "" {
		exec.Command("iptables", "-D", chainName, "-o", k.vpnInterface, "-j", "ACCEPT").CombinedOutput()
	}
	k.vpnInterface = interfaceName
	cmd := exec.Command("iptables", "-I", chainName, "4", "-o", interfaceName, "-j", "ACCEPT")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update VPN interface: %w: %s", err, string(out))
	}
	return nil
}
