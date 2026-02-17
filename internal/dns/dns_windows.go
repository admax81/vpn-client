//go:build windows

package dns

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/user/vpn-client/internal/procutil"
)

// Configure configures DNS for the VPN interface (Windows).
func (m *Manager) Configure(cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.interfaceName = cfg.InterfaceName
	m.vpnDNS = cfg.Servers
	m.splitDNS = cfg.SplitDNS
	m.splitDomains = cfg.Domains

	if err := m.saveOriginalDNS(); err != nil {
		// Non-fatal
	}

	if len(cfg.Servers) > 0 {
		if err := m.setInterfaceDNS(cfg.InterfaceName, cfg.Servers); err != nil {
			return fmt.Errorf("failed to set interface DNS: %w", err)
		}
	}

	if cfg.SplitDNS && len(cfg.Domains) > 0 {
		if err := m.configureNRPT(cfg.Domains, cfg.Servers); err != nil {
			return fmt.Errorf("failed to configure NRPT: %w", err)
		}
	}

	return nil
}

func (m *Manager) saveOriginalDNS() error {
	cmd := procutil.HideWindow(exec.Command("powershell", "-Command",
		fmt.Sprintf(`(Get-DnsClientServerAddress -InterfaceAlias "%s" -AddressFamily IPv4).ServerAddresses -join ","`, m.interfaceName)))
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	dns := strings.TrimSpace(string(out))
	if dns != "" {
		m.originalDNS = strings.Split(dns, ",")
	}

	return nil
}

func (m *Manager) setInterfaceDNS(interfaceName string, servers []string) error {
	cmd := procutil.HideWindow(exec.Command("netsh", "interface", "ipv4", "set", "dnsservers",
		fmt.Sprintf("name=%s", interfaceName),
		"source=static",
		fmt.Sprintf("address=%s", servers[0]),
		"validate=no",
	))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set primary DNS: %w: %s", err, string(out))
	}

	for i := 1; i < len(servers); i++ {
		cmd = procutil.HideWindow(exec.Command("netsh", "interface", "ipv4", "add", "dnsservers",
			fmt.Sprintf("name=%s", interfaceName),
			fmt.Sprintf("address=%s", servers[i]),
			"validate=no",
		))
		cmd.CombinedOutput()
	}

	return nil
}

func (m *Manager) configureNRPT(domains []string, dnsServers []string) error {
	for _, domain := range domains {
		if !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}

		dnsStr := strings.Join(dnsServers, ",")
		cmd := procutil.HideWindow(exec.Command("powershell", "-Command",
			fmt.Sprintf(`Add-DnsClientNrptRule -Namespace "%s" -NameServers %s -Comment "VPN Client Split DNS"`,
				domain, dnsStr)))

		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add NRPT rule for %s: %w: %s", domain, err, string(out))
		}
	}

	return nil
}

// Reset restores original DNS configuration (Windows).
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.removeNRPTRules()

	if len(m.originalDNS) > 0 && m.interfaceName != "" {
		m.setInterfaceDNS(m.interfaceName, m.originalDNS)
	}

	m.originalDNS = nil
	return nil
}

func (m *Manager) removeNRPTRules() error {
	cmd := procutil.HideWindow(exec.Command("powershell", "-Command",
		`Get-DnsClientNrptRule | Where-Object { $_.Comment -eq "VPN Client Split DNS" } | Remove-DnsClientNrptRule -Force`))
	cmd.CombinedOutput()
	return nil
}

// FlushDNSCache flushes the DNS cache (Windows).
func (m *Manager) FlushDNSCache() error {
	cmd := procutil.HideWindow(exec.Command("ipconfig", "/flushdns"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to flush DNS cache: %w: %s", err, string(out))
	}
	return nil
}

// EnableDNSLeakProtection blocks DNS queries to non-VPN DNS servers (Windows).
func (m *Manager) EnableDNSLeakProtection(vpnDNS []string) error {
	m.DisableDNSLeakProtection()

	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=VPNClient_BlockDNS",
		"dir=out", "action=block", "protocol=udp", "remoteport=53",
	))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add DNS block rule: %w: %s", err, string(out))
	}

	for i, dns := range vpnDNS {
		cmd = procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			fmt.Sprintf("name=VPNClient_AllowVPNDNS_%d", i),
			"dir=out", "action=allow", "protocol=udp", "remoteport=53",
			fmt.Sprintf("remoteip=%s", dns),
		))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add DNS allow rule: %w: %s", err, string(out))
		}
	}

	return nil
}

// DisableDNSLeakProtection removes DNS leak firewall rules (Windows).
func (m *Manager) DisableDNSLeakProtection() error {
	cmd := procutil.HideWindow(exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name=VPNClient_BlockDNS"))
	cmd.CombinedOutput()
	return nil
}
