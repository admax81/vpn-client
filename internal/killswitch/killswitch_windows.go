//go:build windows

package killswitch

import (
	"fmt"
	"os/exec"

	"github.com/user/vpn-client/internal/procutil"
)

const rulePrefix = "VPNClient_KillSwitch"

// Enable activates the kill switch (Windows — netsh advfirewall).
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

	if err := k.blockAllOutbound(); err != nil {
		return fmt.Errorf("failed to block outbound: %w", err)
	}
	if err := k.allowLoopback(); err != nil {
		k.disableUnsafe()
		return fmt.Errorf("failed to allow loopback: %w", err)
	}
	if err := k.allowDHCP(); err != nil {
		k.disableUnsafe()
		return fmt.Errorf("failed to allow DHCP: %w", err)
	}
	if cfg.VPNServerIP != "" {
		if err := k.allowVPNServer(cfg.VPNServerIP); err != nil {
			k.disableUnsafe()
			return fmt.Errorf("failed to allow VPN server: %w", err)
		}
	}
	if cfg.VPNInterface != "" {
		if err := k.allowVPNInterface(cfg.VPNInterface); err != nil {
			k.disableUnsafe()
			return fmt.Errorf("failed to allow VPN interface: %w", err)
		}
	}
	if cfg.AllowLAN {
		k.allowLANTraffic()
	}
	for _, proc := range cfg.AllowedProcesses {
		k.allowProcess(proc)
	}

	k.enabled = true
	k.rulesCreated = true
	return nil
}

// Disable deactivates the kill switch (Windows).
func (k *KillSwitch) Disable() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.disableUnsafe()
}

func (k *KillSwitch) disableUnsafe() error {
	rules := []string{
		"BlockAllOutbound", "AllowLoopback", "AllowDHCP",
		"AllowVPNServer", "AllowVPNInterface",
		"AllowLAN_10", "AllowLAN_172", "AllowLAN_192", "AllowLinkLocal",
	}
	for _, rule := range rules {
		procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
			fmt.Sprintf("name=%s_%s", rulePrefix, rule))).CombinedOutput()
	}
	for i := range k.allowedProcs {
		procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
			fmt.Sprintf("name=%s_AllowProcess_%d", rulePrefix, i))).CombinedOutput()
	}

	procutil.HideWindow(exec.Command("netsh", "advfirewall", "set", "allprofiles",
		"firewallpolicy", "blockinbound,allowoutbound")).CombinedOutput()

	k.enabled = false
	k.rulesCreated = false
	return nil
}

// UpdateVPNInterface updates the allowed VPN interface (Windows).
func (k *KillSwitch) UpdateVPNInterface(interfaceName string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.enabled {
		return nil
	}

	procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s_AllowVPNInterface", rulePrefix))).CombinedOutput()

	k.vpnInterface = interfaceName
	return k.allowVPNInterface(interfaceName)
}

func (k *KillSwitch) blockAllOutbound() error {
	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "set", "allprofiles",
		"firewallpolicy", "blockinbound,blockoutbound"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set firewall policy: %w: %s", err, string(out))
	}
	return nil
}

func (k *KillSwitch) allowLoopback() error {
	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s_AllowLoopback", rulePrefix),
		"dir=out", "action=allow", "remoteip=127.0.0.0/8"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to allow loopback: %w: %s", err, string(out))
	}
	return nil
}

func (k *KillSwitch) allowDHCP() error {
	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s_AllowDHCP", rulePrefix),
		"dir=out", "action=allow", "protocol=udp", "localport=68", "remoteport=67"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to allow DHCP: %w: %s", err, string(out))
	}
	return nil
}

func (k *KillSwitch) allowVPNServer(serverIP string) error {
	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s_AllowVPNServer", rulePrefix),
		"dir=out", "action=allow", fmt.Sprintf("remoteip=%s", serverIP)))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to allow VPN server: %w: %s", err, string(out))
	}
	return nil
}

func (k *KillSwitch) allowVPNInterface(interfaceName string) error {
	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s_AllowVPNInterface", rulePrefix),
		"dir=out", "action=allow", fmt.Sprintf("interface=%s", interfaceName)))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to allow VPN interface: %w: %s", err, string(out))
	}
	return nil
}

func (k *KillSwitch) allowLANTraffic() error {
	lanRanges := map[string]string{
		"AllowLAN_10":    "10.0.0.0/8",
		"AllowLAN_172":   "172.16.0.0/12",
		"AllowLAN_192":   "192.168.0.0/16",
		"AllowLinkLocal": "169.254.0.0/16",
	}
	for name, cidr := range lanRanges {
		procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			fmt.Sprintf("name=%s_%s", rulePrefix, name),
			"dir=out", "action=allow", fmt.Sprintf("remoteip=%s", cidr))).CombinedOutput()
	}
	return nil
}

func (k *KillSwitch) allowProcess(processPath string) error {
	index := len(k.allowedProcs)
	procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s_AllowProcess_%d", rulePrefix, index),
		"dir=out", "action=allow", fmt.Sprintf("program=%s", processPath))).CombinedOutput()
	return nil
}
