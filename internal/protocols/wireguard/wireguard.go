// Package wireguard implements the WireGuard VPN protocol.
package wireguard

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/protocols"
	tunpkg "github.com/user/vpn-client/internal/tun"
)

// Tunnel implements the WireGuard VPN tunnel.
type Tunnel struct {
	*protocols.BaseTunnel

	mu       sync.Mutex
	cfg      *config.WireGuard
	ifaceCfg *config.Interface
	adapter  *tunpkg.Adapter
	device   *device.Device
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new WireGuard tunnel.
func New(cfg *config.WireGuard, ifaceCfg *config.Interface) *Tunnel {
	return &Tunnel{
		BaseTunnel: protocols.NewBaseTunnel(),
		cfg:        cfg,
		ifaceCfg:   ifaceCfg,
	}
}

// Start establishes the WireGuard tunnel.
func (t *Tunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State() == protocols.StateConnected {
		return fmt.Errorf("tunnel already connected")
	}

	t.SetState(protocols.StateConnecting, "Initializing WireGuard tunnel", nil)

	t.ctx, t.cancel = context.WithCancel(ctx)

	// Parse configuration
	localAddr, err := netip.ParsePrefix(t.cfg.Address)
	if err != nil {
		t.SetState(protocols.StateError, "Invalid address", err)
		return fmt.Errorf("invalid address: %w", err)
	}
	t.LocalIPAddr = localAddr.Addr()

	// Resolve server endpoint
	host, port, err := net.SplitHostPort(t.cfg.Peer.Endpoint)
	if err != nil {
		t.SetState(protocols.StateError, "Invalid endpoint", err)
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	// Resolve hostname to IP
	ips, err := net.LookupIP(host)
	if err != nil {
		t.SetState(protocols.StateError, "Failed to resolve server", err)
		return fmt.Errorf("failed to resolve server: %w", err)
	}
	if len(ips) == 0 {
		t.SetState(protocols.StateError, "No IP addresses found", nil)
		return fmt.Errorf("no IP addresses found for %s", host)
	}
	t.ServerIPAddr = ips[0].String()

	// Create TUN adapter
	t.adapter, err = tunpkg.New(&tunpkg.Config{
		Name:   t.ifaceCfg.Name,
		MTU:    t.ifaceCfg.MTU,
		Metric: t.ifaceCfg.Metric,
	})
	if err != nil {
		t.SetState(protocols.StateError, "Failed to create adapter config", err)
		return fmt.Errorf("failed to create adapter config: %w", err)
	}

	if err := t.adapter.Create(); err != nil {
		t.SetState(protocols.StateError, "Failed to create TUN adapter", err)
		return fmt.Errorf("failed to create TUN adapter: %w", err)
	}

	// Configure adapter IP
	if err := t.adapter.Configure(t.cfg.Address); err != nil {
		t.adapter.Close()
		t.SetState(protocols.StateError, "Failed to configure adapter", err)
		return fmt.Errorf("failed to configure adapter: %w", err)
	}

	// Create WireGuard device
	tunDevice := t.adapter.Device()
	logger := device.NewLogger(device.LogLevelError, "(wireguard) ")

	t.device = device.NewDevice(tunDevice, conn.NewDefaultBind(), logger)

	// Generate UAPI configuration
	uapiConfig, err := t.generateUAPIConfig(port)
	if err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to generate config", err)
		return fmt.Errorf("failed to generate UAPI config: %w", err)
	}

	// Apply configuration
	if err := t.device.IpcSet(uapiConfig); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to apply config", err)
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Bring device up
	if err := t.device.Up(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to bring device up", err)
		return fmt.Errorf("failed to bring device up: %w", err)
	}

	// Bring adapter up
	if err := t.adapter.Up(); err != nil {
		t.cleanup()
		t.SetState(protocols.StateError, "Failed to bring adapter up", err)
		return fmt.Errorf("failed to bring adapter up: %w", err)
	}

	// Calculate gateway IP (first IP in subnet)
	t.GatewayIPAddr = calculateGateway(localAddr)

	t.SetState(protocols.StateConnected, "WireGuard tunnel established", nil)

	// Start monitoring
	go t.monitor()

	return nil
}

// Stop terminates the WireGuard tunnel.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State() == protocols.StateDisconnected {
		return nil
	}

	t.SetState(protocols.StateDisconnecting, "Stopping WireGuard tunnel", nil)

	if t.cancel != nil {
		t.cancel()
	}

	t.cleanup()

	t.SetState(protocols.StateDisconnected, "WireGuard tunnel stopped", nil)
	t.Close() // Close stateChanges channel so monitorTunnel exits

	return nil
}

// Reconnect attempts to reconnect the tunnel.
func (t *Tunnel) Reconnect() error {
	t.SetState(protocols.StateReconnecting, "Reconnecting", nil)

	if err := t.Stop(); err != nil {
		return err
	}

	// Re-create stateChanges channel (Stop closed it)
	t.ResetChannel()

	return t.Start(context.Background())
}

func (t *Tunnel) cleanup() {
	if t.device != nil {
		t.device.Close()
		t.device = nil
	}

	if t.adapter != nil {
		t.adapter.Down()
		t.adapter.Close()
		t.adapter = nil
	}
}

func (t *Tunnel) generateUAPIConfig(port string) (string, error) {
	// Decode private key
	privateKey, err := base64.StdEncoding.DecodeString(t.cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	// Decode public key
	publicKey, err := base64.StdEncoding.DecodeString(t.cfg.Peer.PublicKey)
	if err != nil {
		return "", fmt.Errorf("invalid public key: %w", err)
	}

	var config strings.Builder

	// Private key
	config.WriteString(fmt.Sprintf("private_key=%s\n", hex.EncodeToString(privateKey)))

	// Peer configuration
	config.WriteString(fmt.Sprintf("public_key=%s\n", hex.EncodeToString(publicKey)))

	// Preshared key (if present)
	if t.cfg.Peer.PresharedKey != "" {
		psk, err := base64.StdEncoding.DecodeString(t.cfg.Peer.PresharedKey)
		if err != nil {
			return "", fmt.Errorf("invalid preshared key: %w", err)
		}
		config.WriteString(fmt.Sprintf("preshared_key=%s\n", hex.EncodeToString(psk)))
	}

	// Endpoint
	config.WriteString(fmt.Sprintf("endpoint=%s:%s\n", t.ServerIPAddr, port))

	// Allowed IPs - only route traffic for configured IPs
	// For split tunneling, we DON'T set 0.0.0.0/0
	// The actual routing is handled by the routing module
	// Here we allow all traffic that arrives at the tunnel
	config.WriteString("allowed_ip=0.0.0.0/0\n")
	config.WriteString("allowed_ip=::/0\n")

	// Persistent keepalive
	if t.cfg.Peer.PersistentKeepalive > 0 {
		config.WriteString(fmt.Sprintf("persistent_keepalive_interval=%d\n", t.cfg.Peer.PersistentKeepalive))
	}

	return config.String(), nil
}

func (t *Tunnel) monitor() {
	<-t.ctx.Done()
}

// GetDevice returns the underlying WireGuard device.
func (t *Tunnel) GetDevice() *device.Device {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.device
}

// GetAdapter returns the TUN adapter.
func (t *Tunnel) GetAdapter() *tunpkg.Adapter {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.adapter
}

// GetTunDevice returns the TUN device.
func (t *Tunnel) GetTunDevice() tun.Device {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.adapter != nil {
		return t.adapter.Device()
	}
	return nil
}

// calculateGateway calculates the gateway IP (typically .1 in the subnet).
func calculateGateway(prefix netip.Prefix) netip.Addr {
	// For point-to-point links, the gateway is typically the remote endpoint
	// We'll use the network address + 1
	addr := prefix.Masked().Addr()
	if addr.Is4() {
		a := addr.As4()
		a[3] = 1
		return netip.AddrFrom4(a)
	}
	return addr
}
