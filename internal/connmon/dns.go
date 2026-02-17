package connmon

import (
	"net"
	"strings"
)

func (m *Monitor) resolveDomainsAsync(conns []Connection) {
	seen := make(map[string]bool)
	for i := range conns {
		ip := conns[i].RemoteAddr.Addr().String()
		if ip == "" || ip == "0.0.0.0" || ip == "::" {
			continue
		}
		if seen[ip] {
			continue
		}
		seen[ip] = true

		m.dnsCacheMu.RLock()
		domain, ok := m.dnsCache[ip]
		m.dnsCacheMu.RUnlock()
		if ok {
			conns[i].Domain = domain
			continue
		}

		// Resolve in background
		go func(ipStr string) {
			names, err := net.LookupAddr(ipStr)
			var domain string
			if err == nil && len(names) > 0 {
				domain = strings.TrimSuffix(names[0], ".")
			}
			m.dnsCacheMu.Lock()
			m.dnsCache[ipStr] = domain
			m.dnsCacheMu.Unlock()
		}(ip)
	}

	// Apply cached domains
	for i := range conns {
		if conns[i].Domain == "" {
			ip := conns[i].RemoteAddr.Addr().String()
			m.dnsCacheMu.RLock()
			conns[i].Domain = m.dnsCache[ip]
			m.dnsCacheMu.RUnlock()
		}
	}
}

func (m *Monitor) lookupCachedDomain(ip string) string {
	m.dnsCacheMu.RLock()
	defer m.dnsCacheMu.RUnlock()
	return m.dnsCache[ip]
}
