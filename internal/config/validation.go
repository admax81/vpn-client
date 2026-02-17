package config

import (
	"fmt"
	"net"
)

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Version < 1 {
		return fmt.Errorf("invalid config version")
	}

	switch c.Protocol {
	case ProtocolWireGuard:
		if err := c.WireGuard.Validate(); err != nil {
			return fmt.Errorf("wireguard config: %w", err)
		}
	case ProtocolOpenVPN:
		if err := c.OpenVPN.Validate(); err != nil {
			return fmt.Errorf("openvpn config: %w", err)
		}
	case ProtocolSSH:
		if err := c.SSH.Validate(); err != nil {
			return fmt.Errorf("ssh config: %w", err)
		}
	default:
		return fmt.Errorf("unknown protocol: %s", c.Protocol)
	}

	if err := c.Routing.Validate(); err != nil {
		return fmt.Errorf("routing config: %w", err)
	}

	if err := c.Interface.Validate(); err != nil {
		return fmt.Errorf("interface config: %w", err)
	}

	return nil
}

// Validate validates WireGuard configuration.
func (w *WireGuard) Validate() error {
	if w.PrivateKey == "" {
		return fmt.Errorf("private_key is required")
	}
	if w.Address == "" {
		return fmt.Errorf("address is required")
	}
	if w.Peer.PublicKey == "" {
		return fmt.Errorf("peer.public_key is required")
	}
	if w.Peer.Endpoint == "" {
		return fmt.Errorf("peer.endpoint is required")
	}
	return nil
}

// Validate validates OpenVPN configuration.
func (o *OpenVPN) Validate() error {
	if o.ConfigPath == "" {
		return fmt.Errorf("config_path is required")
	}
	return nil
}

// Validate validates SSH configuration.
func (s *SSH) Validate() error {
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}
	if s.Port == 0 {
		return fmt.Errorf("port is required")
	}
	if s.User == "" {
		return fmt.Errorf("user is required")
	}
	if s.KeyPath == "" && s.Password == "" {
		return fmt.Errorf("key_path or password is required")
	}
	if s.RemoteTunAddr != "" {
		ip := s.RemoteTunAddr
		if idx := len(ip) - 1; idx >= 0 {
			for i, ch := range ip {
				if ch == '/' {
					ip = ip[:i]
					break
				}
			}
		}
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid remote_tun_addr: %s", s.RemoteTunAddr)
		}
	}
	if s.LocalTunAddr != "" {
		ip := s.LocalTunAddr
		for i, ch := range ip {
			if ch == '/' {
				ip = ip[:i]
				break
			}
		}
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid local_tun_addr: %s", s.LocalTunAddr)
		}
	}
	return nil
}

// Validate validates routing configuration.
func (r *Routing) Validate() error {
	for _, ip := range r.IncludeIPs {
		_, _, err := net.ParseCIDR(ip)
		if err != nil {
			// Try parsing as plain IP
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid IP/CIDR: %s", ip)
			}
		}
	}
	if r.DNSRefreshInterval < 0 {
		return fmt.Errorf("dns_refresh_interval cannot be negative")
	}
	return nil
}

// Validate validates interface configuration.
func (i *Interface) Validate() error {
	if i.Name == "" {
		return fmt.Errorf("name is required")
	}
	if i.MTU < 576 || i.MTU > 65535 {
		return fmt.Errorf("mtu must be between 576 and 65535")
	}
	if i.Metric < 1 || i.Metric > 9999 {
		return fmt.Errorf("metric must be between 1 and 9999")
	}
	return nil
}
