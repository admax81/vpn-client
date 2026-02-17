package ssh

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/protocols"
	tunpkg "github.com/user/vpn-client/internal/tun"
)

// Start establishes the SSH TUN tunnel.
func (t *Tunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State() == protocols.StateConnected {
		return fmt.Errorf("tunnel already connected")
	}

	t.SetState(protocols.StateConnecting, "Initializing SSH tunnel", nil)
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.stopCh = make(chan struct{})

	// Resolve server IP
	serverAddr := fmt.Sprintf("%s:%d", t.cfg.Host, t.cfg.Port)
	ips, err := net.LookupIP(t.cfg.Host)
	if err == nil && len(ips) > 0 {
		t.ServerIPAddr = ips[0].String()
	} else {
		t.ServerIPAddr = t.cfg.Host
	}
	logger.Info("Resolved server IP: %s", t.ServerIPAddr)
	fmt.Printf("SSH LOG: Resolved server IP: %s\n", t.ServerIPAddr)

	// Prepare SSH client config
	sshConfig, err := t.buildSSHConfig()
	if err != nil {
		t.SetState(protocols.StateError, "Failed to build SSH config", err)
		return fmt.Errorf("failed to build SSH config: %w", err)
	}
	logger.Debug("SSH config built successfully")
	fmt.Printf("SSH LOG: SSH config built successfully\n")

	// Connect to SSH server with TCP keepalive
	dialer := net.Dialer{Timeout: 30 * time.Second}
	tcpConn, err := dialer.DialContext(ctx, "tcp", serverAddr)
	if err != nil {
		t.SetState(protocols.StateError, "Failed to connect to SSH server", err)
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Enable TCP-level keepalive on the socket
	if tc, ok := tcpConn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(10 * time.Second)
		logger.Debug("TCP keepalive enabled (10s)")
	}

	// SSH handshake over the established TCP connection
	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, serverAddr, sshConfig)
	if err != nil {
		tcpConn.Close()
		t.SetState(protocols.StateError, "SSH handshake failed", err)
		return fmt.Errorf("SSH handshake failed: %w", err)
	}
	t.client = ssh.NewClient(sshConn, chans, reqs)
	logger.Connection("Connected to SSH server at %s", serverAddr)
	fmt.Printf("SSH LOG: Connected to SSH server at %s\n", serverAddr)

	// Create TUN adapter locally
	t.adapter, err = tunpkg.New(&tunpkg.Config{
		Name:   t.ifaceCfg.Name,
		MTU:    t.ifaceCfg.MTU,
		Metric: t.ifaceCfg.Metric,
	})
	if err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to create adapter config", err)
		return fmt.Errorf("failed to create adapter config: %w", err)
	}

	if err := t.adapter.Create(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to create TUN adapter", err)
		return fmt.Errorf("failed to create TUN adapter: %w", err)
	}
	logger.Info("TUN adapter created: %s", t.ifaceCfg.Name)
	fmt.Printf("SSH LOG: TUN adapter created: %s\n", t.ifaceCfg.Name)

	// Configure adapter IP - use local_tun_addr or default
	localAddr := t.cfg.LocalTunAddr
	if localAddr == "" {
		localAddr = "10.0.0.2/24"
	}
	// Ensure it has CIDR notation
	if !containsSlash(localAddr) {
		localAddr = localAddr + "/24"
	}

	if err := t.adapter.Configure(localAddr); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to configure adapter", err)
		return fmt.Errorf("failed to configure adapter: %w", err)
	}
	logger.Info("TUN adapter configured with IP: %s", localAddr)

	// Parse local IP (strip CIDR)
	localIPOnly := localAddr
	if idx := indexOf(localAddr, '/'); idx > 0 {
		localIPOnly = localAddr[:idx]
	}
	t.LocalIPAddr, _ = netip.ParseAddr(localIPOnly)

	// Parse remote IP
	remoteIP := t.cfg.RemoteTunAddr
	if remoteIP == "" {
		remoteIP = "10.0.0.1"
	}
	t.GatewayIPAddr, _ = netip.ParseAddr(remoteIP)

	// Create SSH session for tunnel
	t.session, err = t.client.NewSession()
	if err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to create SSH session", err)
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	logger.Debug("SSH session created")

	// Request TUN device on remote side
	// This requires PermitTunnel=yes in server sshd_config
	err = t.requestTunnel()
	if err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to request tunnel", err)
		return fmt.Errorf("failed to request tunnel: %w", err)
	}
	logger.Info("Tunnel requested on remote side")
	fmt.Printf("SSH LOG: Tunnel requested on remote side\n")

	// Bring adapter up
	if err := t.adapter.Up(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to bring adapter up", err)
		return fmt.Errorf("failed to bring adapter up: %w", err)
	}
	logger.Info("TUN adapter brought up")

	// Start packet forwarding
	if err := t.startForwarding(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to start forwarding", err)
		return fmt.Errorf("failed to start forwarding: %w", err)
	}
	logger.Info("Packet forwarding started")

	t.SetState(protocols.StateConnected, "SSH tunnel established", nil)
	logger.Connection("SSH tunnel fully established")
	fmt.Printf("SSH LOG: SSH tunnel fully established\n")

	// Start keepalive
	go t.keepalive()

	return nil
}

// Stop terminates the SSH tunnel.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State() == protocols.StateDisconnected {
		return nil
	}

	t.SetState(protocols.StateDisconnecting, "Stopping SSH tunnel", nil)

	if t.cancel != nil {
		t.cancel()
	}

	// Signal goroutines to stop (safe against double-close)
	t.stopOnce.Do(func() { close(t.stopCh) })

	// Close connections FIRST to unblock goroutines waiting on I/O
	// (adapter.Read, io.ReadFull on SSH stdout, etc.)
	t.cleanup()

	// Now wait for goroutines with a timeout as safety net
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		logger.Warning("SSH tunnel goroutines did not exit within 5s timeout")
	}

	t.SetState(protocols.StateDisconnected, "SSH tunnel stopped", nil)
	t.Close() // Close stateChanges channel so monitorTunnel exits

	return nil
}

// Reconnect attempts to reconnect the tunnel.
func (t *Tunnel) Reconnect() error {
	t.SetState(protocols.StateReconnecting, "Reconnecting", nil)

	if err := t.Stop(); err != nil {
		return err
	}

	// Reset stopOnce for new lifecycle
	t.stopOnce = sync.Once{}

	// Re-create stateChanges channel (Stop closed it)
	t.ResetChannel()

	return t.Start(context.Background())
}
