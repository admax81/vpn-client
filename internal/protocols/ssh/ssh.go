// Package ssh implements VPN over SSH using TUN tunneling.
// This file is deprecated - code split into multiple files:
// - tunnel.go: Tunnel struct, New, helpers
// - connection.go: Start, Stop, Reconnect
// - authconfig.go: buildSSHConfig
// - remote.go: requestTunnel
// - bridge.go: startPacketBridge, startForwarding
// - keepalive.go: keepalive
package ssh
