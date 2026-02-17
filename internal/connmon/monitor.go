package connmon

import (
	"sync"
	"time"
)

// Monitor tracks connections and detects connection state changes.
type Monitor struct {
	mu              sync.Mutex
	running         bool
	stopCh          chan struct{}
	connections     []Connection
	journal         []JournalEntry
	maxJournal      int
	dnsCache        map[string]string // IP -> domain reverse lookup cache
	dnsCacheMu      sync.RWMutex
	prevSynSent     map[string]time.Time // key -> first seen time for SYN_SENT tracking
	prevSynProcess  map[string]string    // key -> process name for SYN_SENT tracking
	prevEstablished map[string]bool      // key -> true if already logged as ESTABLISHED
	synTimeout      time.Duration
	pollInterval    time.Duration
	onUpdate        func() // callback when data changes
}

// NewMonitor creates a new connection monitor.
func NewMonitor() *Monitor {
	return &Monitor{
		stopCh:          make(chan struct{}),
		maxJournal:      5000,
		dnsCache:        make(map[string]string),
		prevSynSent:     make(map[string]time.Time),
		prevSynProcess:  make(map[string]string),
		prevEstablished: make(map[string]bool),
		synTimeout:      10 * time.Second,
		pollInterval:    2 * time.Second,
	}
}

// SetOnUpdate sets a callback that fires when connection data changes.
func (m *Monitor) SetOnUpdate(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = fn
}

// Start begins monitoring connections.
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go m.pollLoop()
}

// Stop stops the monitor.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
}

// GetConnections returns a snapshot of current connections.
func (m *Monitor) GetConnections() []Connection {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Connection, len(m.connections))
	copy(result, m.connections)
	return result
}

// GetJournal returns a snapshot of the journal entries.
func (m *Monitor) GetJournal() []JournalEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]JournalEntry, len(m.journal))
	copy(result, m.journal)
	return result
}

// ClearJournal clears the journal.
func (m *Monitor) ClearJournal() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.journal = nil
}

func (m *Monitor) pollLoop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
		}
	}
}
