// Package connmon provides connection monitoring for route discovery.
// It tracks active TCP/UDP connections and logs failed connection attempts
// (DROP/TIMEOUT) to help discover routes that may need to be added to VPN routing.
//
// This file is deprecated - code split into multiple files:
// - types.go: ConnState, Connection, JournalEntry
// - monitor.go: Monitor struct, lifecycle methods
// - detection.go: Connection failure detection
// - dns.go: Domain resolution
// - sort.go: Connection sorting utilities
package connmon
