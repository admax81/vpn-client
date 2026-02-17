// Package protocols defines the common interface for VPN protocols.
package protocols

import (
	"context"
	"net/netip"
	"sync"
)

// State represents the tunnel state.
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateDisconnecting
	StateReconnecting
	StateError
)

func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnecting:
		return "disconnecting"
	case StateReconnecting:
		return "reconnecting"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Stats represents tunnel statistics.
type Stats struct {
	BytesSent     uint64
	BytesReceived uint64
	PacketsSent   uint64
	PacketsRecv   uint64
}

// StateChange represents a state change event.
type StateChange struct {
	State   State
	Message string
	Error   error
}

// Tunnel is the interface that all VPN protocols must implement.
type Tunnel interface {
	// Start establishes the VPN tunnel.
	Start(ctx context.Context) error

	// Stop terminates the VPN tunnel.
	Stop() error

	// State returns the current tunnel state.
	State() State

	// Stats returns current tunnel statistics.
	Stats() Stats

	// LocalIP returns the local tunnel IP address.
	LocalIP() netip.Addr

	// GatewayIP returns the tunnel gateway IP address.
	GatewayIP() netip.Addr

	// ServerIP returns the VPN server IP address.
	ServerIP() string

	// StateChanges returns a channel for state change notifications.
	StateChanges() <-chan StateChange

	// Reconnect attempts to reconnect the tunnel.
	Reconnect() error
}

// BaseTunnel provides common functionality for tunnel implementations.
type BaseTunnel struct {
	stateMu       sync.Mutex
	state         State
	stateChanges  chan StateChange
	closed        bool
	closeChanOnce sync.Once
	LocalIPAddr   netip.Addr
	GatewayIPAddr netip.Addr
	ServerIPAddr  string
	Statistics    Stats
}

// NewBaseTunnel creates a new base tunnel.
func NewBaseTunnel() *BaseTunnel {
	return &BaseTunnel{
		state:        StateDisconnected,
		stateChanges: make(chan StateChange, 16),
	}
}

// SetState sets the tunnel state and notifies listeners.
func (b *BaseTunnel) SetState(state State, message string, err error) {
	b.stateMu.Lock()
	b.state = state
	closed := b.closed
	b.stateMu.Unlock()

	if closed {
		return
	}

	select {
	case b.stateChanges <- StateChange{State: state, Message: message, Error: err}:
	default:
		// Channel full, drop the message
	}
}

// State returns the current state.
func (b *BaseTunnel) State() State {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.state
}

// StateChanges returns the state changes channel.
func (b *BaseTunnel) StateChanges() <-chan StateChange {
	return b.stateChanges
}

// Stats returns current statistics.
func (b *BaseTunnel) Stats() Stats {
	return b.Statistics
}

// LocalIP returns the local IP.
func (b *BaseTunnel) LocalIP() netip.Addr {
	return b.LocalIPAddr
}

// GatewayIP returns the gateway IP.
func (b *BaseTunnel) GatewayIP() netip.Addr {
	return b.GatewayIPAddr
}

// ServerIP returns the server IP.
func (b *BaseTunnel) ServerIP() string {
	return b.ServerIPAddr
}

// Close closes the state changes channel. Safe to call multiple times.
func (b *BaseTunnel) Close() {
	b.closeChanOnce.Do(func() {
		b.stateMu.Lock()
		b.closed = true
		b.stateMu.Unlock()
		close(b.stateChanges)
	})
}

// ResetChannel re-creates the state changes channel for reuse after Close.
// Must be called before Start() when reconnecting a stopped tunnel.
func (b *BaseTunnel) ResetChannel() {
	b.stateMu.Lock()
	b.stateChanges = make(chan StateChange, 16)
	b.closed = false
	b.closeChanOnce = sync.Once{}
	b.stateMu.Unlock()
}
