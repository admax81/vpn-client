// Package config handles VPN client configuration loading, saving, and validation.
package config

// Protocol represents the VPN protocol type.
type Protocol string

const (
	ProtocolWireGuard Protocol = "wireguard"
	ProtocolOpenVPN   Protocol = "openvpn"
	ProtocolSSH       Protocol = "ssh"
)

// Config represents the main configuration structure.
type Config struct {
	Version    int              `yaml:"version"`
	Protocol   Protocol         `yaml:"protocol"`
	Autostart  bool             `yaml:"autostart"`
	WireGuard  WireGuard        `yaml:"wireguard,omitempty"`
	OpenVPN    OpenVPN          `yaml:"openvpn,omitempty"`
	SSH        SSH              `yaml:"ssh,omitempty"`
	Routing    Routing          `yaml:"routing"`
	DNS        DNS              `yaml:"dns"`
	Interface  Interface        `yaml:"interface"`
	KillSwitch KillSwitchConfig `yaml:"killswitch"`
}

// WireGuard configuration.
type WireGuard struct {
	PrivateKey string        `yaml:"private_key"`
	Address    string        `yaml:"address"`
	DNS        string        `yaml:"dns,omitempty"`
	Peer       WireGuardPeer `yaml:"peer"`
}

// WireGuardPeer represents a WireGuard peer configuration.
type WireGuardPeer struct {
	PublicKey           string `yaml:"public_key"`
	Endpoint            string `yaml:"endpoint"`
	PersistentKeepalive int    `yaml:"persistent_keepalive,omitempty"`
	PresharedKey        string `yaml:"preshared_key,omitempty"`
}

// OpenVPN configuration.
type OpenVPN struct {
	ConfigPath string `yaml:"config_path,omitempty"`
	AuthUser   string `yaml:"auth_user,omitempty"`
	AuthPass   string `yaml:"auth_pass,omitempty"`
}

// SSH tunnel configuration.
type SSH struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	User              string `yaml:"user"`
	KeyPath           string `yaml:"key_path,omitempty"`
	Password          string `yaml:"password,omitempty"`
	RemoteTunAddr     string `yaml:"remote_tun_addr,omitempty"`
	LocalTunAddr      string `yaml:"local_tun_addr,omitempty"`
	RoutingFile       string `yaml:"routing_file,omitempty"`       // Path to routing file on the remote server
	KeepAliveInterval int    `yaml:"keepalive_interval,omitempty"` // seconds, 0 = use default (10)
	KeepAliveRetries  int    `yaml:"keepalive_retries,omitempty"`  // missed pings before reconnect, 0 = use default (3)
}

// Routing configuration for split tunneling.
type Routing struct {
	DefaultRoute       bool     `yaml:"default_route"` // Route all traffic through VPN
	IncludeIPs         []string `yaml:"include_ips"`
	IncludeDomains     []string `yaml:"include_domains"`
	DNSRefreshInterval int      `yaml:"dns_refresh_interval"` // seconds
}

// DNS configuration.
type DNS struct {
	Servers  []string `yaml:"servers"`
	SplitDNS bool     `yaml:"split_dns"`
	Domains  []string `yaml:"domains,omitempty"`
}

// Interface configuration for the VPN adapter.
type Interface struct {
	Name   string `yaml:"name"`
	MTU    int    `yaml:"mtu"`
	Metric int    `yaml:"metric"`
}

// KillSwitchConfig represents kill switch configuration.
type KillSwitchConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowLAN         bool     `yaml:"allow_lan"`
	AllowedProcesses []string `yaml:"allowed_processes,omitempty"`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		Version:   1,
		Protocol:  ProtocolWireGuard,
		Autostart: false,
		WireGuard: WireGuard{
			Address: "10.0.0.2/24",
			Peer: WireGuardPeer{
				Endpoint:            "vpn.example.com:51820",
				PersistentKeepalive: 25,
			},
		},
		OpenVPN: OpenVPN{},
		SSH: SSH{
			Port:              22,
			LocalTunAddr:      "10.0.0.2/24",
			RemoteTunAddr:     "10.0.0.1",
			KeepAliveInterval: 10,
			KeepAliveRetries:  3,
		},
		Routing: Routing{
			DefaultRoute:       false,
			IncludeIPs:         []string{},
			IncludeDomains:     []string{},
			DNSRefreshInterval: 300,
		},
		DNS: DNS{
			Servers:  []string{"1.1.1.1", "8.8.8.8"},
			SplitDNS: true,
			Domains:  []string{},
		},
		Interface: Interface{
			Name:   "VPNClient",
			MTU:    1420,
			Metric: 5,
		},
		KillSwitch: KillSwitchConfig{
			Enabled:  false,
			AllowLAN: true,
		},
	}
}
