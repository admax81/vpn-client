package connmon

import (
	"fmt"
	"net/netip"
	"time"
)

// ConnState represents the state of a TCP connection.
type ConnState int

const (
	StateClosed      ConnState = 1
	StateListen      ConnState = 2
	StateSynSent     ConnState = 3
	StateSynReceived ConnState = 4
	StateEstablished ConnState = 5
	StateFinWait1    ConnState = 6
	StateFinWait2    ConnState = 7
	StateCloseWait   ConnState = 8
	StateClosing     ConnState = 9
	StateLastAck     ConnState = 10
	StateTimeWait    ConnState = 11
)

// String returns a human-readable name for the connection state.
func (s ConnState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateListen:
		return "LISTEN"
	case StateSynSent:
		return "SYN_SENT"
	case StateSynReceived:
		return "SYN_RECV"
	case StateEstablished:
		return "ESTABLISHED"
	case StateFinWait1:
		return "FIN_WAIT1"
	case StateFinWait2:
		return "FIN_WAIT2"
	case StateCloseWait:
		return "CLOSE_WAIT"
	case StateClosing:
		return "CLOSING"
	case StateLastAck:
		return "LAST_ACK"
	case StateTimeWait:
		return "TIME_WAIT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

// Connection represents an active network connection.
type Connection struct {
	Protocol    string // "TCP" or "UDP"
	LocalAddr   netip.AddrPort
	RemoteAddr  netip.AddrPort
	State       ConnState // Only meaningful for TCP
	PID         uint32
	ProcessName string // Executable name resolved from PID
	Domain      string // Resolved domain name (reverse DNS)
}

// Key returns a unique key for this connection.
func (c *Connection) Key() string {
	return fmt.Sprintf("%s|%s|%s", c.Protocol, c.LocalAddr, c.RemoteAddr)
}

// JournalEntry represents a connection attempt logged in the journal.
type JournalEntry struct {
	Timestamp   time.Time
	Protocol    string
	LocalAddr   netip.AddrPort
	RemoteAddr  netip.AddrPort
	Domain      string
	Reason      string // "ESTABLISHED", "SYN_TIMEOUT", "CONNECTION_REFUSED", "DROPPED", etc.
	RemoteIP    string // For easy display
	ProcessName string // Executable name that initiated the connection
}
