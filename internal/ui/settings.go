package ui

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	vpnconfig "github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/logger"
)

// AppConfig represents the application configuration for the settings UI.
type AppConfig struct {
	Version   int    `yaml:"version"`
	Protocol  string `yaml:"protocol"`
	Autostart bool   `yaml:"autostart"`

	WireGuard struct {
		PrivateKey string `yaml:"private_key"`
		Address    string `yaml:"address"`
		Peer       struct {
			PublicKey           string `yaml:"public_key"`
			Endpoint            string `yaml:"endpoint"`
			PersistentKeepalive int    `yaml:"persistent_keepalive"`
		} `yaml:"peer"`
	} `yaml:"wireguard"`

	OpenVPN struct {
		ConfigPath string `yaml:"config_path"`
		AuthUser   string `yaml:"auth_user"`
		AuthPass   string `yaml:"auth_pass"`
	} `yaml:"openvpn"`

	SSH struct {
		Host          string `yaml:"host"`
		Port          int    `yaml:"port"`
		User          string `yaml:"user"`
		KeyPath       string `yaml:"key_path,omitempty"`
		RemoteTunAddr string `yaml:"remote_tun_addr,omitempty"`
		LocalTunAddr  string `yaml:"local_tun_addr,omitempty"`
	} `yaml:"ssh"`

	Routing struct {
		DefaultRoute       bool     `yaml:"default_route"`
		IncludeIPs         []string `yaml:"include_ips"`
		IncludeDomains     []string `yaml:"include_domains"`
		DNSRefreshInterval int      `yaml:"dns_refresh_interval"`
	} `yaml:"routing"`

	DNS struct {
		Servers  []string `yaml:"servers"`
		SplitDNS bool     `yaml:"split_dns"`
		Domains  []string `yaml:"domains"`
	} `yaml:"dns"`

	Interface struct {
		Name   string `yaml:"name"`
		MTU    int    `yaml:"mtu"`
		Metric int    `yaml:"metric"`
	} `yaml:"interface"`

	Killswitch struct {
		Enabled  bool `yaml:"enabled"`
		AllowLAN bool `yaml:"allow_lan"`
	} `yaml:"killswitch"`
}

func loadAppConfig() *AppConfig {
	cfg := &AppConfig{
		Version:   1,
		Protocol:  "wireguard",
		Autostart: false,
	}
	cfg.WireGuard.Address = "10.255.0.2/24"
	cfg.WireGuard.Peer.PersistentKeepalive = 25
	cfg.SSH.Port = 22
	cfg.Routing.DNSRefreshInterval = 300
	cfg.Interface.Name = "VPNClient"
	cfg.Interface.MTU = 1420
	cfg.Interface.Metric = 5
	cfg.Killswitch.AllowLAN = true

	configPath := vpnconfig.GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err == nil {
		yaml.Unmarshal(data, cfg)
	}

	return cfg
}

func saveAppConfig(cfg *AppConfig) error {
	configPath := vpnconfig.GetConfigPath()
	dir := filepath.Dir(configPath)
	os.MkdirAll(dir, 0755)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func clearLogFile() {
	logger.ClearLogs()
}
