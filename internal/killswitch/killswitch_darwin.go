//go:build darwin

package killswitch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const pfAnchor = "com.vpnclient.killswitch"

// Enable activates the kill switch (macOS — pf firewall).
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

	// Build pf rules
	var rules strings.Builder
	rules.WriteString("# VPN Client Kill Switch\n")

	// Allow loopback
	rules.WriteString("pass quick on lo0 all\n")

	// Allow DHCP
	rules.WriteString("pass out quick proto udp from any port 68 to any port 67\n")

	// Allow VPN server
	if cfg.VPNServerIP != "" {
		rules.WriteString(fmt.Sprintf("pass out quick to %s\n", cfg.VPNServerIP))
	}

	// Allow VPN interface
	if cfg.VPNInterface != "" {
		rules.WriteString(fmt.Sprintf("pass quick on %s all\n", cfg.VPNInterface))
	}

	// Allow LAN
	if cfg.AllowLAN {
		for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16"} {
			rules.WriteString(fmt.Sprintf("pass out quick to %s\n", cidr))
		}
	}

	// Block everything else
	rules.WriteString("block out all\n")

	// Write rules to temp file
	rulesFile := "/tmp/vpnclient_pf.conf"
	if err := os.WriteFile(rulesFile, []byte(rules.String()), 0600); err != nil {
		return fmt.Errorf("failed to write pf rules: %w", err)
	}

	// Load rules
	cmd := exec.Command("pfctl", "-f", rulesFile, "-e")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable pf: %w: %s", err, string(out))
	}

	k.enabled = true
	k.rulesCreated = true
	return nil
}

// Disable deactivates the kill switch (macOS).
func (k *KillSwitch) Disable() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.disableUnsafe()
}

func (k *KillSwitch) disableUnsafe() error {
	// Restore default pf rules
	exec.Command("pfctl", "-f", "/etc/pf.conf").CombinedOutput()

	// Remove temp file
	os.Remove("/tmp/vpnclient_pf.conf")

	k.enabled = false
	k.rulesCreated = false
	return nil
}

// UpdateVPNInterface updates the allowed VPN interface (macOS).
func (k *KillSwitch) UpdateVPNInterface(interfaceName string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.enabled {
		return nil
	}

	k.vpnInterface = interfaceName

	// Re-enable with updated config
	k.disableUnsafe()
	k.mu.Unlock()
	err := k.Enable(&Config{
		Enabled:          true,
		AllowLAN:         k.allowLAN,
		VPNServerIP:      k.vpnServerIP,
		VPNInterface:     interfaceName,
		AllowedProcesses: k.allowedProcs,
	})
	k.mu.Lock()
	return err
}
