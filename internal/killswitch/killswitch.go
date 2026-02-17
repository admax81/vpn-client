// Package killswitch implements a network kill switch to prevent traffic leaks.
package killswitch

import (
	"sync"
)

// KillSwitch manages firewall rules to prevent traffic leaks.
type KillSwitch struct {
	mu           sync.Mutex
	enabled      bool
	allowLAN     bool
	vpnServerIP  string
	vpnInterface string
	allowedProcs []string
	rulesCreated bool
}

// Config represents kill switch configuration.
type Config struct {
	Enabled          bool
	AllowLAN         bool
	VPNServerIP      string
	VPNInterface     string
	AllowedProcesses []string
}

// New creates a new kill switch manager.
func New() *KillSwitch {
	return &KillSwitch{}
}

// IsEnabled returns whether the kill switch is enabled.
func (k *KillSwitch) IsEnabled() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.enabled
}
