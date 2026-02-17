// Package openvpn implements the OpenVPN VPN protocol.
package openvpn

import (
	"context"
	"net"
	"os"
	"os/exec"
	"sync"

	"github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/protocols"
)

const (
	// Default management interface port
	managementPort = 17166
)

// Tunnel implements the OpenVPN VPN tunnel.
type Tunnel struct {
	*protocols.BaseTunnel

	mu           sync.Mutex
	cfg          *config.OpenVPN
	ifaceCfg     *config.Interface
	process      *exec.Cmd
	mgmtConn     net.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	tempConfPath string
	connected    chan struct{}
}

// New creates a new OpenVPN tunnel.
func New(cfg *config.OpenVPN, ifaceCfg *config.Interface) *Tunnel {
	return &Tunnel{
		BaseTunnel: protocols.NewBaseTunnel(),
		cfg:        cfg,
		ifaceCfg:   ifaceCfg,
		connected:  make(chan struct{}),
	}
}

// GetProcess returns the OpenVPN process.
func (t *Tunnel) GetProcess() *exec.Cmd {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.process
}

func (t *Tunnel) cleanup() {
	if t.mgmtConn != nil {
		t.mgmtConn.Close()
		t.mgmtConn = nil
	}

	if t.process != nil && t.process.Process != nil {
		t.process.Process.Kill()
		t.process.Wait()
		t.process = nil
	}

	if t.tempConfPath != "" {
		os.Remove(t.tempConfPath)
		t.tempConfPath = ""
	}

	// Reset connected channel
	select {
	case <-t.connected:
	default:
	}
	t.connected = make(chan struct{})
}
