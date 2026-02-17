//go:build windows

package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/lxn/walk"

	"github.com/user/vpn-client/internal/connmon"
	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/routing"
)

// --- Local routes cache for journal highlighting ---

var (
	localRoutesCacheMu       sync.RWMutex
	localRoutesCachePrefixes []netip.Prefix
	localRoutesCacheIPs      []netip.Addr
	localRoutesCacheDomains  []string // lowercase
)

// refreshLocalRoutesCache reloads the local routes.txt and caches parsed entries.
func refreshLocalRoutesCache() {
	routes, err := routing.ReadLocalRoutesFile()
	if err != nil {
		logger.Warning("Failed to read local routes for cache: " + err.Error())
		return
	}

	var prefixes []netip.Prefix
	var addrs []netip.Addr
	var domains []string

	if routes != nil {
		for _, entry := range routes.IPs {
			entry = strings.TrimSpace(entry)
			if p, err := netip.ParsePrefix(entry); err == nil {
				prefixes = append(prefixes, p)
			} else if addr, err := netip.ParseAddr(entry); err == nil {
				addrs = append(addrs, addr)
			}
		}
		for _, d := range routes.Domains {
			domains = append(domains, strings.ToLower(strings.TrimSpace(d)))
		}
	}

	localRoutesCacheMu.Lock()
	localRoutesCachePrefixes = prefixes
	localRoutesCacheIPs = addrs
	localRoutesCacheDomains = domains
	localRoutesCacheMu.Unlock()
}

// isIPInLocalRoutes checks if an IP address matches any entry in the local routes cache.
func isIPInLocalRoutes(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	localRoutesCacheMu.RLock()
	defer localRoutesCacheMu.RUnlock()

	for _, p := range localRoutesCachePrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	for _, a := range localRoutesCacheIPs {
		if a == addr {
			return true
		}
	}
	return false
}

// isDomainInLocalRoutes checks if a domain matches any entry in the local routes cache.
func isDomainInLocalRoutes(domain string) bool {
	if domain == "" {
		return false
	}
	domain = strings.ToLower(domain)
	localRoutesCacheMu.RLock()
	defer localRoutesCacheMu.RUnlock()

	for _, d := range localRoutesCacheDomains {
		if d == domain {
			return true
		}
		// Wildcard match: *.example.com matches sub.example.com
		if strings.HasPrefix(d, "*.") {
			suffix := d[1:] // ".example.com"
			if strings.HasSuffix(domain, suffix) || domain == d[2:] {
				return true
			}
		}
	}
	return false
}

// isJournalEntryInRoutes checks if a journal entry (by IP or domain) matches routes.txt.
func isJournalEntryInRoutes(e connmon.JournalEntry) bool {
	if e.RemoteIP != "" {
		if addr, err := netip.ParseAddr(e.RemoteIP); err == nil {
			if isIPInLocalRoutes(addr) {
				return true
			}
		}
	}
	if isDomainInLocalRoutes(e.Domain) {
		return true
	}
	return false
}

// findMatchingRouteEntries returns route entries from routes.txt that match the journal entry.
// It reads the file directly so the returned strings can be used for deletion.
func findMatchingRouteEntries(e connmon.JournalEntry) []string {
	routes, err := routing.ReadLocalRoutesFile()
	if err != nil || routes == nil {
		return nil
	}

	var matches []string
	addr, addrOK := netip.Addr{}, false
	if e.RemoteIP != "" {
		if a, err := netip.ParseAddr(e.RemoteIP); err == nil {
			addr = a
			addrOK = true
		}
	}

	// Check IP/CIDR entries
	for _, entry := range routes.IPs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if p, err := netip.ParsePrefix(entry); err == nil {
			if addrOK && p.Contains(addr) {
				matches = append(matches, entry)
			}
		} else if a, err := netip.ParseAddr(entry); err == nil {
			if addrOK && a == addr {
				matches = append(matches, entry)
			}
		}
	}

	// Check domain entries
	if e.Domain != "" {
		domLower := strings.ToLower(e.Domain)
		for _, d := range routes.Domains {
			d = strings.TrimSpace(d)
			dl := strings.ToLower(d)
			if dl == domLower {
				matches = append(matches, d)
			} else if strings.HasPrefix(dl, "*.") {
				suffix := dl[1:]
				if strings.HasSuffix(domLower, suffix) || domLower == dl[2:] {
					matches = append(matches, d)
				}
			}
		}
	}

	return matches
}

// --- CIDR lookup via RDAP API ---

var (
	rdapCacheMu sync.RWMutex
	rdapCache   = make(map[string]rdapCacheEntry) // IP string -> cached result
	rdapClient  = &http.Client{Timeout: 5 * time.Second}
)

const (
	rdapCacheTTL     = 24 * time.Hour
	rdapBootstrapURL = "https://rdap-bootstrap.arin.net/bootstrap/ip/"
)

type rdapCacheEntry struct {
	cidr    string
	fetched time.Time
}

// rdapResponse is a minimal struct for parsing RDAP IP network responses.
type rdapResponse struct {
	CIDR0CIDRs []rdapCIDR `json:"cidr0_cidrs"`
}

type rdapCIDR struct {
	V4Prefix string `json:"v4prefix"`
	V6Prefix string `json:"v6prefix"`
	Length   int    `json:"length"`
}

// ipToCIDR resolves an IP address to its CIDR via RDAP (cached).
// Returns empty string if lookup fails or IP is invalid.
func ipToCIDR(addr netip.Addr) (result string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ipToCIDR panic for %v: %v", addr, r)
			result = ""
		}
	}()

	if !addr.IsValid() {
		return ""
	}

	ipStr := addr.String()

	// Check cache first
	rdapCacheMu.RLock()
	if entry, ok := rdapCache[ipStr]; ok && time.Since(entry.fetched) < rdapCacheTTL {
		rdapCacheMu.RUnlock()
		return entry.cidr
	}
	rdapCacheMu.RUnlock()

	// Fetch from RDAP asynchronously — return empty for now, cache result for next refresh
	go fetchRDAPCIDR(ipStr)
	return ""
}

// fetchRDAPCIDR fetches CIDR for an IP from RDAP and stores in cache.
func fetchRDAPCIDR(ipStr string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("fetchRDAPCIDR panic for %s: %v", ipStr, r)
		}
	}()

	// Double-check cache to avoid duplicate requests
	rdapCacheMu.RLock()
	if entry, ok := rdapCache[ipStr]; ok && time.Since(entry.fetched) < rdapCacheTTL {
		rdapCacheMu.RUnlock()
		return
	}
	rdapCacheMu.RUnlock()

	url := rdapBootstrapURL + ipStr
	resp, err := rdapClient.Get(url)
	if err != nil {
		logger.Warning("RDAP request failed for %s: %v", ipStr, err)
		cacheRDAPResult(ipStr, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warning("RDAP returned status %d for %s", resp.StatusCode, ipStr)
		cacheRDAPResult(ipStr, "")
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		logger.Warning("RDAP read body failed for %s: %v", ipStr, err)
		cacheRDAPResult(ipStr, "")
		return
	}

	var rdap rdapResponse
	if err := json.Unmarshal(body, &rdap); err != nil {
		logger.Warning("RDAP JSON parse failed for %s: %v", ipStr, err)
		cacheRDAPResult(ipStr, "")
		return
	}

	cidr := ""
	if len(rdap.CIDR0CIDRs) > 0 {
		c := rdap.CIDR0CIDRs[0]
		prefix := c.V4Prefix
		if prefix == "" {
			prefix = c.V6Prefix
		}
		if prefix != "" && c.Length > 0 {
			cidr = fmt.Sprintf("%s/%d", prefix, c.Length)
		}
	}

	cacheRDAPResult(ipStr, cidr)
}

func cacheRDAPResult(ipStr, cidr string) {
	rdapCacheMu.Lock()
	rdapCache[ipStr] = rdapCacheEntry{cidr: cidr, fetched: time.Now()}
	rdapCacheMu.Unlock()
}

// --- Routing table model ---

type cmRoutingTableModel struct {
	walk.TableModelBase
	items []*cmRouteItem
}

type cmRouteItem struct {
	Type  string // "IP/CIDR" or "Domain"
	Value string
}

func (m *cmRoutingTableModel) RowCount() int {
	return len(m.items)
}

func (m *cmRoutingTableModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return nil
	}
	item := m.items[row]
	switch col {
	case 0:
		return item.Type
	case 1:
		return item.Value
	}
	return nil
}

func (m *cmRoutingTableModel) loadFromRemote(routes *routing.RemoteRoutes) {
	m.items = nil
	if routes == nil {
		m.PublishRowsReset()
		return
	}
	for _, ip := range routes.IPs {
		m.items = append(m.items, &cmRouteItem{Type: "IP/CIDR", Value: ip})
	}
	for _, domain := range routes.Domains {
		m.items = append(m.items, &cmRouteItem{Type: "Domain", Value: domain})
	}
	m.PublishRowsReset()
}

// --- Connection table model ---

type connMonConnModel struct {
	walk.TableModelBase
	items []connmon.Connection
}

func (m *connMonConnModel) RowCount() int {
	return len(m.items)
}

func (m *connMonConnModel) Value(row, col int) interface{} {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("connMonConnModel.Value panic row=%d col=%d: %v", row, col, r)
		}
	}()

	if row < 0 || row >= len(m.items) {
		return nil
	}
	c := m.items[row]
	switch col {
	case 0:
		return c.Protocol
	case 1:
		return c.RemoteAddr.Addr().String()
	case 2:
		return fmt.Sprintf("%d", c.RemoteAddr.Port())
	case 3:
		cidr := ipToCIDR(c.RemoteAddr.Addr())
		switch {
		case cidr != "" && c.Domain != "":
			return fmt.Sprintf("%s (%s)", cidr, c.Domain)
		case cidr != "":
			return cidr
		case c.Domain != "":
			return c.Domain
		default:
			return "—"
		}
	case 4:
		if c.Protocol == "TCP" {
			return c.State.String()
		}
		return "—"
	case 5:
		return c.LocalAddr.String()
	case 6:
		if c.ProcessName != "" {
			return c.ProcessName
		}
		if c.PID > 0 {
			return fmt.Sprintf("PID %d", c.PID)
		}
		return "—"
	}
	return nil
}

func (m *connMonConnModel) update(conns []connmon.Connection) {
	// Filter local IPs and deduplicate by remote IP
	filtered := make([]connmon.Connection, 0, len(conns))
	seenRemote := make(map[string]bool)

	for _, c := range conns {
		remoteIP := c.RemoteAddr.Addr()
		remoteStr := remoteIP.String()

		// Skip localhost and loopback
		if remoteIP.IsLoopback() {
			continue
		}

		// Skip private networks: 192.168.*.*, 10.*.*.*
		if remoteIP.Is4() {
			octets := remoteIP.As4()
			// 192.168.0.0/16
			if octets[0] == 192 && octets[1] == 168 {
				continue
			}
			// 10.0.0.0/8
			if octets[0] == 10 {
				continue
			}
		}

		// Skip if we've already seen this remote IP
		if seenRemote[remoteStr] {
			continue
		}
		seenRemote[remoteStr] = true

		filtered = append(filtered, c)
	}

	connmon.SortConnectionsByState(filtered)
	m.items = filtered
	m.PublishRowsReset()
}

// --- Journal table model ---

type connMonJournalModel struct {
	walk.TableModelBase
	items []connmon.JournalEntry
}

func (m *connMonJournalModel) RowCount() int {
	return len(m.items)
}

func (m *connMonJournalModel) Value(row, col int) interface{} {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("connMonJournalModel.Value panic row=%d col=%d: %v", row, col, r)
		}
	}()

	if row < 0 || row >= len(m.items) {
		return nil
	}
	e := m.items[row]
	switch col {
	case 0:
		return e.Timestamp.Format("15:04:05")
	case 1:
		return e.Protocol
	case 2:
		return e.RemoteIP
	case 3:
		if e.RemoteAddr.Port() > 0 {
			return fmt.Sprintf("%d", e.RemoteAddr.Port())
		}
		return "—"
	case 4:
		// Parse RemoteIP and convert to CIDR
		var cidr string
		if e.RemoteIP != "" {
			if addr, err := netip.ParseAddr(e.RemoteIP); err == nil {
				cidr = ipToCIDR(addr)
			}
		}
		switch {
		case cidr != "" && e.Domain != "":
			return fmt.Sprintf("%s (%s)", cidr, e.Domain)
		case cidr != "":
			return cidr
		case e.Domain != "":
			return e.Domain
		default:
			return "—"
		}
	case 5:
		return e.Reason
	case 6:
		if e.ProcessName != "" {
			return e.ProcessName
		}
		return "—"
	}
	return nil
}

func (m *connMonJournalModel) update(entries []connmon.JournalEntry) {
	// Reverse order: newest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	m.items = entries
	m.PublishRowsReset()
}
