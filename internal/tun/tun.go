// Package tun provides cross-platform TUN interface management.
package tun

import (
	"fmt"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

// Adapter represents a TUN adapter.
type Adapter struct {
	mu      sync.Mutex
	name    string
	device  tun.Device
	localIP netip.Addr
	subnet  netip.Prefix
	mtu     int
	metric  int
	ifIndex uint32
	isUp    bool
}

// Config represents TUN adapter configuration.
type Config struct {
	Name    string
	Address string // CIDR notation, e.g., "10.255.0.2/24"
	MTU     int
	Metric  int
}

// New creates a new TUN adapter.
func New(cfg *Config) (*Adapter, error) {
	if cfg.Name == "" {
		cfg.Name = "VPNClient"
	}
	if cfg.MTU == 0 {
		cfg.MTU = 1420
	}
	if cfg.Metric == 0 {
		cfg.Metric = 5
	}

	return &Adapter{
		name:   normalizeInterfaceName(cfg.Name),
		mtu:    cfg.MTU,
		metric: cfg.Metric,
	}, nil
}

// Create creates the TUN device.
func (a *Adapter) Create() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device != nil {
		return fmt.Errorf("adapter already created")
	}

	device, err := tun.CreateTUN(a.name, a.mtu)
	if err != nil {
		return fmt.Errorf("failed to create TUN device: %w", err)
	}
	a.device = device

	realName, err := device.Name()
	if err == nil {
		a.name = realName
	}

	return nil
}

// Configure configures the adapter with IP address.
func (a *Adapter) Configure(address string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}

	a.localIP = prefix.Addr()
	a.subnet = prefix

	if err := a.assignIP(prefix); err != nil {
		return err
	}

	if err := a.setMetric(a.metric); err != nil {
		return err
	}

	return nil
}

// Close closes and destroys the adapter.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.device != nil {
		a.device.Close()
		a.device = nil
	}

	a.isUp = false
	return nil
}

// Device returns the underlying TUN device.
func (a *Adapter) Device() tun.Device {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.device
}

// Name returns the adapter name.
func (a *Adapter) Name() string {
	return a.name
}

// LocalIP returns the local IP address.
func (a *Adapter) LocalIP() netip.Addr {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.localIP
}

// InterfaceIndex returns the interface index.
func (a *Adapter) InterfaceIndex() uint32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ifIndex
}

// MTU returns the MTU.
func (a *Adapter) MTU() int {
	return a.mtu
}

// IsUp returns whether the adapter is up.
func (a *Adapter) IsUp() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isUp
}

// Read reads a packet from the TUN device.
func (a *Adapter) Read(buf []byte, offset int) (int, error) {
	if a.device == nil {
		return 0, fmt.Errorf("adapter not created")
	}
	sizes := make([]int, 1)
	n, err := a.device.Read([][]byte{buf[offset:]}, sizes, offset)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	return sizes[0], nil
}

// Write writes a packet to the TUN device.
func (a *Adapter) Write(buf []byte, offset int) (int, error) {
	if a.device == nil {
		return 0, fmt.Errorf("adapter not created")
	}
	return a.device.Write([][]byte{buf[offset:]}, offset)
}
