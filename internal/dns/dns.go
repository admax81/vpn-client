// Package dns provides DNS management for VPN split tunneling.
package dns

import (
	"sync"
)

// Manager manages DNS configuration for VPN.
type Manager struct {
	mu            sync.Mutex
	interfaceName string
	originalDNS   []string
	vpnDNS        []string
	splitDNS      bool
	splitDomains  []string
}

// Config represents DNS configuration.
type Config struct {
	Servers       []string
	SplitDNS      bool
	Domains       []string
	InterfaceName string
}

// NewManager creates a new DNS manager.
func NewManager() *Manager {
	return &Manager{}
}
