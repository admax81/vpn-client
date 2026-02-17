// Package ssh implements VPN over SSH using TUN tunneling.
package ssh

import (
	"context"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/protocols"
	tunpkg "github.com/user/vpn-client/internal/tun"
)

// Tunnel implements the SSH TUN VPN tunnel.
type Tunnel struct {
	*protocols.BaseTunnel

	mu       sync.Mutex
	cfg      *config.SSH
	ifaceCfg *config.Interface
	client   *ssh.Client
	session  *ssh.Session
	adapter  *tunpkg.Adapter
	ctx      context.Context
	cancel   context.CancelFunc
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// New creates a new SSH tunnel.
func New(cfg *config.SSH, ifaceCfg *config.Interface) *Tunnel {
	return &Tunnel{
		BaseTunnel: protocols.NewBaseTunnel(),
		cfg:        cfg,
		ifaceCfg:   ifaceCfg,
		stopCh:     make(chan struct{}),
	}
}

// GetAdapter returns the TUN adapter.
func (t *Tunnel) GetAdapter() *tunpkg.Adapter {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.adapter
}

// GetClient returns the SSH client.
func (t *Tunnel) GetClient() *ssh.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.client
}

// Helper functions
func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}

func indexOf(s string, c rune) int {
	for i, ch := range s {
		if ch == c {
			return i
		}
	}
	return -1
}

func (t *Tunnel) cleanup() {
	if t.session != nil {
		t.session.Close()
		t.session = nil
	}

	if t.client != nil {
		t.client.Close()
		t.client = nil
	}

	if t.adapter != nil {
		t.adapter.Down()
		t.adapter.Close()
		t.adapter = nil
	}
}
