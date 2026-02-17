package core

import (
	"fmt"
	"net/netip"

	"github.com/user/vpn-client/internal/config"
)

// validateConfig validates the configuration before connecting.
func (s *Service) validateConfig(cfg *config.Config) error {
	switch cfg.Protocol {
	case config.ProtocolWireGuard:
		if cfg.WireGuard.PrivateKey == "" {
			return fmt.Errorf("WireGuard: private key is required")
		}
		if cfg.WireGuard.Address == "" {
			return fmt.Errorf("WireGuard: address is required (e.g., 10.0.0.2/24)")
		}
		if cfg.WireGuard.Peer.PublicKey == "" {
			return fmt.Errorf("WireGuard: peer public key is required")
		}
		if cfg.WireGuard.Peer.Endpoint == "" {
			return fmt.Errorf("WireGuard: peer endpoint is required (e.g., server:51820)")
		}
		// Validate address format
		if _, err := netip.ParsePrefix(cfg.WireGuard.Address); err != nil {
			return fmt.Errorf("WireGuard: invalid address format '%s' (expected CIDR like 10.0.0.2/24)", cfg.WireGuard.Address)
		}

	case config.ProtocolOpenVPN:
		if cfg.OpenVPN.ConfigPath == "" {
			return fmt.Errorf("OpenVPN: config file path is required")
		}

	case config.ProtocolSSH:
		if cfg.SSH.Host == "" {
			return fmt.Errorf("SSH: host is required")
		}
		if cfg.SSH.User == "" {
			return fmt.Errorf("SSH: user is required")
		}
		if cfg.SSH.KeyPath == "" && cfg.SSH.Password == "" {
			return fmt.Errorf("SSH: key_path or password is required")
		}

	default:
		return fmt.Errorf("unknown protocol: %s", cfg.Protocol)
	}

	return nil
}

// getServerIP extracts server IP from configuration.
func (s *Service) getServerIP(cfg *config.Config) string {
	switch cfg.Protocol {
	case config.ProtocolWireGuard:
		// Extract host from endpoint
		endpoint := cfg.WireGuard.Peer.Endpoint
		host, _, _ := splitHostPort(endpoint)
		return host
	case config.ProtocolOpenVPN:
		// Parse from config file if needed
		return ""
	case config.ProtocolSSH:
		return cfg.SSH.Host
	}
	return ""
}

// splitHostPort splits host:port string.
func splitHostPort(hostport string) (host, port string, err error) {
	// Simple split without net package to avoid circular import
	for i := len(hostport) - 1; i >= 0; i-- {
		if hostport[i] == ':' {
			return hostport[:i], hostport[i+1:], nil
		}
	}
	return hostport, "", nil
}
