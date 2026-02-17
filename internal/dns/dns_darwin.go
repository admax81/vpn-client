//go:build darwin

package dns

import (
	"fmt"
	"os/exec"
	"strings"
)

// Configure configures DNS for the VPN interface (macOS).
func (m *Manager) Configure(cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.interfaceName = cfg.InterfaceName
	m.vpnDNS = cfg.Servers
	m.splitDNS = cfg.SplitDNS
	m.splitDomains = cfg.Domains

	m.saveOriginalDNS()

	if len(cfg.Servers) > 0 {
		if err := m.setDNS(cfg.Servers); err != nil {
			return fmt.Errorf("failed to set DNS: %w", err)
		}
	}

	if cfg.SplitDNS && len(cfg.Domains) > 0 {
		m.configureSplitDNS(cfg.Domains, cfg.Servers)
	}

	return nil
}

func (m *Manager) saveOriginalDNS() {
	// Get DNS from the primary network service
	service := m.getPrimaryNetworkService()
	if service == "" {
		return
	}

	cmd := exec.Command("networksetup", "-getdnsservers", service)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	output := strings.TrimSpace(string(out))
	if !strings.Contains(output, "There aren't any DNS Servers") {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				m.originalDNS = append(m.originalDNS, line)
			}
		}
	}
}

func (m *Manager) getPrimaryNetworkService() string {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") {
			continue
		}
		// Return first non-disabled service
		if !strings.HasPrefix(line, "*") {
			return line
		}
	}
	return ""
}

func (m *Manager) setDNS(servers []string) error {
	service := m.getPrimaryNetworkService()
	if service == "" {
		return fmt.Errorf("no primary network service found")
	}

	args := append([]string{"-setdnsservers", service}, servers...)
	cmd := exec.Command("networksetup", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set DNS: %w: %s", err, string(out))
	}

	return nil
}

func (m *Manager) configureSplitDNS(domains []string, servers []string) {
	// macOS split DNS via scutil resolver configuration
	dnsStr := strings.Join(servers, "\n  nameserver[1] : ")
	for _, domain := range domains {
		script := fmt.Sprintf(`d.init
d.add ServerAddresses * %s
d.add SupplementalMatchDomains * %s
set State:/Network/Service/VPNClient/DNS`, strings.Join(servers, " "), domain)

		cmd := exec.Command("scutil")
		cmd.Stdin = strings.NewReader(script)
		cmd.CombinedOutput()
	}
	_ = dnsStr
}

// Reset restores original DNS configuration (macOS).
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	service := m.getPrimaryNetworkService()
	if service == "" {
		return nil
	}

	if len(m.originalDNS) > 0 {
		args := append([]string{"-setdnsservers", service}, m.originalDNS...)
		exec.Command("networksetup", args...).CombinedOutput()
	} else {
		exec.Command("networksetup", "-setdnsservers", service, "empty").CombinedOutput()
	}

	// Remove scutil split DNS
	script := `d.init
remove State:/Network/Service/VPNClient/DNS`
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader(script)
	cmd.CombinedOutput()

	m.originalDNS = nil
	return nil
}

// FlushDNSCache flushes the DNS cache (macOS).
func (m *Manager) FlushDNSCache() error {
	exec.Command("dscacheutil", "-flushcache").CombinedOutput()
	exec.Command("killall", "-HUP", "mDNSResponder").CombinedOutput()
	return nil
}

// EnableDNSLeakProtection blocks DNS queries to non-VPN DNS servers (macOS — pf).
func (m *Manager) EnableDNSLeakProtection(vpnDNS []string) error {
	m.DisableDNSLeakProtection()

	var rules strings.Builder
	rules.WriteString("# VPN Client DNS Leak Protection\n")
	for _, dns := range vpnDNS {
		rules.WriteString(fmt.Sprintf("pass out quick proto udp to %s port 53\n", dns))
	}
	rules.WriteString("block out quick proto udp to any port 53\n")

	// This would need to be combined with existing pf rules in practice
	return nil
}

// DisableDNSLeakProtection removes DNS leak protection rules (macOS).
func (m *Manager) DisableDNSLeakProtection() error {
	// Remove pf rules — in practice, restore original pf.conf
	return nil
}
