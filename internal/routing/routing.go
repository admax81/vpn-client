// Package routing implements split tunneling through routing table manipulation.
package routing

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Route represents a routing table entry.
type Route struct {
	Destination netip.Prefix
	Gateway     netip.Addr
	Interface   uint32 // Interface index
	Metric      int
	Source      string // "static", "domain", "manual", "tunnel"
	Domain      string // Original domain if resolved from domain
}

// Manager manages routing table entries for split tunneling.
type Manager struct {
	mu             sync.Mutex
	routes         map[string]*Route // key: destination string
	vpnGateway     netip.Addr
	vpnIfIndex     uint32
	originalGW     netip.Addr
	originalIfIdx  uint32
	domainResolver *DomainResolver
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewManager creates a new routing manager.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		routes: make(map[string]*Route),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Initialize sets up the routing manager with VPN interface details.
func (m *Manager) Initialize(vpnGateway netip.Addr, vpnIfIndex uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.vpnGateway = vpnGateway
	m.vpnIfIndex = vpnIfIndex

	gw, ifIdx, err := m.getDefaultGateway()
	if err != nil {
		return fmt.Errorf("failed to get default gateway: %w", err)
	}
	m.originalGW = gw
	m.originalIfIdx = ifIdx

	return nil
}

// AddRoute adds a route through the VPN interface.
func (m *Manager) AddRoute(destination string, source string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.addRouteUnsafe(destination, source, "")
}

func (m *Manager) addRouteUnsafe(destination, source, domain string) error {
	prefix, err := netip.ParsePrefix(destination)
	if err != nil {
		addr, err := netip.ParseAddr(destination)
		if err != nil {
			return fmt.Errorf("invalid destination: %s", destination)
		}
		if addr.Is4() {
			prefix = netip.PrefixFrom(addr, 32)
		} else {
			prefix = netip.PrefixFrom(addr, 128)
		}
	}

	key := prefix.String()
	if _, exists := m.routes[key]; exists {
		return nil
	}

	route := &Route{
		Destination: prefix,
		Gateway:     m.vpnGateway,
		Interface:   m.vpnIfIndex,
		Metric:      1,
		Source:      source,
		Domain:      domain,
	}

	if err := m.addSystemRoute(route); err != nil {
		return err
	}

	m.routes[key] = route
	return nil
}

// AddDomainRoute resolves a domain and adds routes for its IPs.
func (m *Manager) AddDomainRoute(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ips, err := net.LookupIP(domain)
	if err != nil {
		return fmt.Errorf("failed to resolve domain %s: %w", domain, err)
	}

	for _, ip := range ips {
		addr, err := netip.ParseAddr(ip.String())
		if err != nil {
			continue
		}
		var prefix netip.Prefix
		if addr.Is4() {
			prefix = netip.PrefixFrom(addr, 32)
		} else {
			prefix = netip.PrefixFrom(addr, 128)
		}
		if err := m.addRouteUnsafe(prefix.String(), "domain", domain); err != nil {
			continue
		}
	}

	return nil
}

// RemoveRoute removes a specific route.
func (m *Manager) RemoveRoute(destination string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	prefix, err := netip.ParsePrefix(destination)
	if err != nil {
		addr, err := netip.ParseAddr(destination)
		if err != nil {
			return fmt.Errorf("invalid destination: %s", destination)
		}
		if addr.Is4() {
			prefix = netip.PrefixFrom(addr, 32)
		} else {
			prefix = netip.PrefixFrom(addr, 128)
		}
	}

	key := prefix.String()
	route, exists := m.routes[key]
	if !exists {
		return nil
	}

	if err := m.removeSystemRoute(route); err != nil {
		return err
	}

	delete(m.routes, key)
	return nil
}

// RemoveAllRoutes removes all routes added by this manager.
func (m *Manager) RemoveAllRoutes() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, route := range m.routes {
		m.removeSystemRoute(route)
	}

	m.routes = make(map[string]*Route)
}

// StartDomainResolver starts periodic domain resolution.
func (m *Manager) StartDomainResolver(domains []string, interval time.Duration) {
	m.domainResolver = &DomainResolver{
		domains:  domains,
		interval: interval,
		manager:  m,
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Domain resolver panicked - silently recover
			}
		}()
		m.domainResolver.Run(m.ctx)
	}()
}

// StopDomainResolver stops the domain resolver.
func (m *Manager) StopDomainResolver() {
	m.cancel()
}

// GetRoutes returns all managed routes.
func (m *Manager) GetRoutes() []*Route {
	m.mu.Lock()
	defer m.mu.Unlock()

	routes := make([]*Route, 0, len(m.routes))
	for _, r := range m.routes {
		routes = append(routes, r)
	}
	return routes
}

// Close cleans up all routes and stops the manager.
func (m *Manager) Close() {
	m.cancel()
	m.RemoveAllRoutes()
}

// GetVPNGateway returns the VPN gateway address.
func (m *Manager) GetVPNGateway() netip.Addr {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vpnGateway
}

// GetOriginalGateway returns the original default gateway.
func (m *Manager) GetOriginalGateway() netip.Addr {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.originalGW
}

// DomainResolver periodically resolves domains and updates routes.
type DomainResolver struct {
	domains  []string
	interval time.Duration
	manager  *Manager
}

// Run starts the domain resolution loop.
func (r *DomainResolver) Run(ctx context.Context) {
	r.resolveAll()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.resolveAll()
		}
	}
}

func (r *DomainResolver) resolveAll() {
	for _, domain := range r.domains {
		if strings.HasPrefix(domain, "*.") {
			baseDomain := strings.TrimPrefix(domain, "*.")
			r.manager.AddDomainRoute(baseDomain)
		} else {
			r.manager.AddDomainRoute(domain)
		}
	}
}

// CIDRMaskString converts a prefix length to a dotted mask string (IPv4 only).
func CIDRMaskString(prefix netip.Prefix) string {
	if prefix.Addr().Is6() {
		return fmt.Sprintf("/%d", prefix.Bits())
	}
	mask := net.CIDRMask(prefix.Bits(), 32)
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
}
