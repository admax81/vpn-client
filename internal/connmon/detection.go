package connmon

import (
	"net/netip"
	"strings"
	"time"
)

func (m *Monitor) poll() {
	conns, err := getSystemConnections()
	if err != nil {
		return
	}

	// Resolve process names
	resolveProcessNames(conns)

	// Resolve domains in background
	m.resolveDomainsAsync(conns)

	// Detect connection state changes (failed and successful)
	m.detectConnectionChanges(conns)

	m.mu.Lock()
	m.connections = conns
	cb := m.onUpdate
	m.mu.Unlock()

	if cb != nil {
		cb()
	}
}

func (m *Monitor) detectConnectionChanges(current []Connection) {
	now := time.Now()
	currentSynSent := make(map[string]bool)
	currentEstablished := make(map[string]bool)

	// Track current ESTABLISHED connections and add to journal if new
	for _, c := range current {
		if c.Protocol == "TCP" && c.State == StateEstablished {
			key := c.Key()
			currentEstablished[key] = true

			m.mu.Lock()
			if !m.prevEstablished[key] {
				// New ESTABLISHED connection - add to journal
				m.addJournalEntry(JournalEntry{
					Timestamp:   now,
					Protocol:    c.Protocol,
					LocalAddr:   c.LocalAddr,
					RemoteAddr:  c.RemoteAddr,
					Domain:      m.lookupCachedDomain(c.RemoteAddr.Addr().String()),
					Reason:      "ESTABLISHED",
					RemoteIP:    c.RemoteAddr.Addr().String(),
					ProcessName: c.ProcessName,
				})
				m.prevEstablished[key] = true
			}
			m.mu.Unlock()
		}
	}

	// Track current SYN_SENT connections
	for _, c := range current {
		if c.Protocol == "TCP" && c.State == StateSynSent {
			key := c.Key()
			currentSynSent[key] = true

			m.mu.Lock()
			if _, exists := m.prevSynSent[key]; !exists {
				m.prevSynSent[key] = now
				m.prevSynProcess[key] = c.ProcessName
			} else if now.Sub(m.prevSynSent[key]) > m.synTimeout {
				// This SYN_SENT has been pending too long — log it
				m.addJournalEntry(JournalEntry{
					Timestamp:   now,
					Protocol:    c.Protocol,
					LocalAddr:   c.LocalAddr,
					RemoteAddr:  c.RemoteAddr,
					Domain:      m.lookupCachedDomain(c.RemoteAddr.Addr().String()),
					Reason:      "SYN_TIMEOUT",
					RemoteIP:    c.RemoteAddr.Addr().String(),
					ProcessName: c.ProcessName,
				})
				// Reset so we don't log again immediately
				m.prevSynSent[key] = now
			}
			m.mu.Unlock()
		}
	}

	// Check connections that were SYN_SENT last time but are now gone (not established)
	m.mu.Lock()
	for key, firstSeen := range m.prevSynSent {
		if !currentSynSent[key] {
			// Connection disappeared — check if it was a failure
			wasEstablished := false
			for _, c := range current {
				if c.Key() == key && c.State == StateEstablished {
					wasEstablished = true
					break
				}
			}
			if !wasEstablished && now.Sub(firstSeen) > 3*time.Second {
				// Parse the key to get addresses
				parts := strings.SplitN(key, "|", 3)
				if len(parts) == 3 {
					if remote, err := netip.ParseAddrPort(parts[2]); err == nil {
						local, _ := netip.ParseAddrPort(parts[1])
						m.addJournalEntry(JournalEntry{
							Timestamp:   now,
							Protocol:    parts[0],
							LocalAddr:   local,
							RemoteAddr:  remote,
							Domain:      m.lookupCachedDomain(remote.Addr().String()),
							Reason:      "DROPPED",
							RemoteIP:    remote.Addr().String(),
							ProcessName: m.prevSynProcess[key],
						})
					}
				}
			}
			delete(m.prevSynSent, key)
			delete(m.prevSynProcess, key)
		}
	}

	// Clean up ESTABLISHED map for connections that are no longer established
	for key := range m.prevEstablished {
		if !currentEstablished[key] {
			delete(m.prevEstablished, key)
		}
	}
	m.mu.Unlock()
}

func (m *Monitor) addJournalEntry(entry JournalEntry) {
	remoteIP := entry.RemoteAddr.Addr()

	// Filter out local and private IPs
	if remoteIP.IsLoopback() {
		return // Skip localhost and 127.0.0.1
	}

	if remoteIP.Is4() {
		octets := remoteIP.As4()
		// Skip 192.168.0.0/16
		if octets[0] == 192 && octets[1] == 168 {
			return
		}
		// Skip 10.0.0.0/8
		if octets[0] == 10 {
			return
		}
	}

	// Check for duplicate recent entries (same remote within last 30s)
	for i := len(m.journal) - 1; i >= 0 && i >= len(m.journal)-20; i-- {
		e := m.journal[i]
		if e.RemoteAddr == entry.RemoteAddr &&
			e.Protocol == entry.Protocol &&
			entry.Timestamp.Sub(e.Timestamp) < 30*time.Second {
			return // Skip duplicate
		}
	}

	m.journal = append(m.journal, entry)
	if len(m.journal) > m.maxJournal {
		m.journal = m.journal[len(m.journal)-m.maxJournal:]
	}
}
