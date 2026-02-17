package core

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/dns"
	"github.com/user/vpn-client/internal/killswitch"
	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/procutil"
	"github.com/user/vpn-client/internal/protocols"
	"github.com/user/vpn-client/internal/protocols/openvpn"
	"github.com/user/vpn-client/internal/protocols/ssh"
	"github.com/user/vpn-client/internal/protocols/wireguard"
	"github.com/user/vpn-client/internal/routing"
)

// Connect establishes the VPN connection.
func (s *Service) Connect() error {
	s.mu.Lock()
	if s.state == StateConnected || s.state == StateConnecting {
		s.mu.Unlock()
		logger.Warning("Connection attempt ignored: already connected or connecting")
		return fmt.Errorf("already connected or connecting")
	}
	s.state = StateConnecting
	s.mu.Unlock()

	s.broadcastStatus()

	cfg := s.configManager.Get()

	// Validate configuration before connecting
	if err := s.validateConfig(cfg); err != nil {
		logger.Error("Configuration validation failed: " + err.Error())
		s.setError(err)
		return err
	}

	logger.Connection(fmt.Sprintf("Initiating %s connection...", cfg.Protocol))

	// Create tunnel based on protocol
	var tunnel protocols.Tunnel
	switch cfg.Protocol {
	case config.ProtocolWireGuard:
		logger.Info("Creating WireGuard tunnel")
		tunnel = wireguard.New(&cfg.WireGuard, &cfg.Interface)
	case config.ProtocolOpenVPN:
		logger.Info("Creating OpenVPN tunnel")
		tunnel = openvpn.New(&cfg.OpenVPN, &cfg.Interface)
	case config.ProtocolSSH:
		logger.Info("Creating SSH tunnel")
		tunnel = ssh.New(&cfg.SSH, &cfg.Interface)
	default:
		logger.Error("Unknown protocol: " + string(cfg.Protocol))
		s.setError(fmt.Errorf("unknown protocol: %s", cfg.Protocol))
		return s.lastError
	}

	// Enable kill switch before connecting
	if cfg.KillSwitch.Enabled {
		logger.Info("Enabling kill switch...")
		serverIP := s.getServerIP(cfg)
		if err := s.killSwitch.Enable(&killswitch.Config{
			Enabled:          true,
			AllowLAN:         cfg.KillSwitch.AllowLAN,
			VPNServerIP:      serverIP,
			VPNInterface:     cfg.Interface.Name,
			AllowedProcesses: cfg.KillSwitch.AllowedProcesses,
		}); err != nil {
			logger.Error("Failed to enable kill switch: " + err.Error())
			s.setError(fmt.Errorf("failed to enable kill switch: %w", err))
			return s.lastError
		}
		logger.Info("Kill switch enabled")
	}

	// Start tunnel
	logger.Info("Starting tunnel...")
	if err := tunnel.Start(s.ctx); err != nil {
		logger.Error("Failed to start tunnel: " + err.Error())
		if cfg.KillSwitch.Enabled {
			s.killSwitch.Disable()
		}
		s.setError(err)
		return err
	}

	s.mu.Lock()
	s.tunnel = tunnel
	s.mu.Unlock()

	// Wait for tunnel to connect
	logger.Info("Waiting for tunnel connection...")
	for i := 0; i < 30; i++ {
		state := tunnel.State()
		logger.Debug("Tunnel state check %d/30: %s", i+1, state)
		if state == protocols.StateConnected {
			break
		}
		if tunnel.State() == protocols.StateError {
			logger.Error("Tunnel connection failed")
			s.setError(fmt.Errorf("tunnel connection failed"))
			s.cleanupOnError(cfg)
			return s.lastError
		}
		time.Sleep(time.Second)
	}

	if tunnel.State() != protocols.StateConnected {
		logger.Error("Connection timeout after 30 seconds, final state: %s", tunnel.State())
		s.setError(fmt.Errorf("connection timeout"))
		s.cleanupOnError(cfg)
		return s.lastError
	}

	logger.Info("Tunnel connected, configuring routing...")

	// Initialize routing
	// For point-to-point interface, use the local tunnel IP as the gateway
	// This ensures packets go through the VPN interface directly
	vpnGateway := tunnel.LocalIP()

	// Get interface index from tunnel local IP
	ifIndex, err := s.routing.GetInterfaceIndexByIP(tunnel.LocalIP().String())
	if err != nil {
		logger.Warning("Failed to get VPN interface index by IP, using 0: " + err.Error())
		ifIndex = 0
	}

	if err := s.routing.Initialize(vpnGateway, ifIndex); err != nil {
		s.setError(fmt.Errorf("failed to initialize routing: %w", err))
		s.cleanupOnError(cfg)
		return err
	}

	// Ensure VPN server is routed via original gateway
	if err := s.routing.EnsureVPNServerRoute(tunnel.ServerIP()); err != nil {
		// Non-fatal, log and continue
	}

	// Remove any existing routes to tunnel gateway
	cmd := procutil.HideWindow(exec.Command("route", "delete", tunnel.GatewayIP().String()))
	cmd.CombinedOutput() // Ignore errors

	// Add route for tunnel gateway
	if err := s.routing.AddRoute(tunnel.GatewayIP().String(), "tunnel"); err != nil {
		logger.Warning("Failed to add tunnel gateway route: " + err.Error())
	}

	// Check if default route is enabled
	if cfg.Routing.DefaultRoute {
		// Route all traffic through VPN using 0.0.0.0/1 and 128.0.0.0/1
		// This approach is more reliable on Windows than single 0.0.0.0/0
		logger.Info("Default route enabled: routing all traffic through VPN")
		if err := s.routing.AddRoute("0.0.0.0/1", "default"); err != nil {
			logger.Warning("Failed to add default route 0.0.0.0/1: " + err.Error())
		}
		if err := s.routing.AddRoute("128.0.0.0/1", "default"); err != nil {
			logger.Warning("Failed to add default route 128.0.0.0/1: " + err.Error())
		}
	} else {
		// Add routes for split tunneling
		logger.Info("Split tunneling mode: routing only specified IPs through VPN")
		for _, route := range cfg.Routing.IncludeIPs {
			if err := s.routing.AddRoute(route, "static"); err != nil {
				// Log but continue
			}
		}
	}

	// Fetch routes from local routes.txt file
	logger.Info("Reading local routes file")
	localRoutes, err := routing.ReadLocalRoutesFile()
	if err != nil {
		logger.Warning("Failed to read local routes file: " + err.Error())
	} else {
		logger.Info(fmt.Sprintf("Loaded %d IPs and %d domains from local routes file",
			len(localRoutes.IPs), len(localRoutes.Domains)))
		for _, ip := range localRoutes.IPs {
			if err := s.routing.AddRoute(ip, "remote"); err != nil {
				logger.Warning("Failed to add route " + ip + ": " + err.Error())
			}
		}
		// Append domains to the list for domain resolver
		cfg.Routing.IncludeDomains = append(cfg.Routing.IncludeDomains, localRoutes.Domains...)
	}

	// Start domain resolver
	if len(cfg.Routing.IncludeDomains) > 0 {
		interval := time.Duration(cfg.Routing.DNSRefreshInterval) * time.Second
		if interval < time.Minute {
			interval = 5 * time.Minute
		}
		s.routing.StartDomainResolver(cfg.Routing.IncludeDomains, interval)
	}

	// Configure DNS
	if len(cfg.DNS.Servers) > 0 {
		logger.Info("Configuring DNS servers: " + fmt.Sprintf("%v", cfg.DNS.Servers))
		if err := s.dns.Configure(&dns.Config{
			Servers:       cfg.DNS.Servers,
			SplitDNS:      cfg.DNS.SplitDNS,
			Domains:       cfg.DNS.Domains,
			InterfaceName: cfg.Interface.Name,
		}); err != nil {
			logger.Warning("DNS configuration failed: " + err.Error())
			// Non-fatal, log and continue
		}

		// Flush DNS cache
		s.dns.FlushDNSCache()
		logger.Info("DNS cache flushed")
	}

	// Update kill switch with VPN interface
	if cfg.KillSwitch.Enabled {
		s.killSwitch.UpdateVPNInterface(cfg.Interface.Name)
	}

	s.mu.Lock()
	s.state = StateConnected
	s.connectedAt = time.Now()
	s.lastError = nil
	s.mu.Unlock()

	logger.Connection(fmt.Sprintf("VPN connected successfully via %s", cfg.Protocol))
	logger.Info(fmt.Sprintf("Local IP: %s, Server: %s", tunnel.LocalIP(), tunnel.ServerIP()))

	s.broadcastStatus()

	// Start monitoring tunnel state changes
	go func() {
		defer logger.Recover("monitorTunnel")
		s.monitorTunnel()
	}()

	return nil
}

// Disconnect terminates the VPN connection.
func (s *Service) Disconnect() error {
	s.mu.Lock()
	if s.state == StateDisconnected || s.state == StateDisconnecting {
		s.mu.Unlock()
		return nil
	}
	s.state = StateDisconnecting
	s.mu.Unlock()

	logger.Connection("Disconnecting VPN...")
	s.broadcastStatus()

	// Run disconnect with a timeout to prevent indefinite hangs
	done := make(chan struct{})
	go func() {
		defer logger.Recover("disconnectInternal")
		s.disconnectInternal()
		close(done)
	}()

	select {
	case <-done:
		// Completed normally
	case <-time.After(15 * time.Second):
		logger.Warning("Disconnect timed out after 15 seconds, forcing state reset")
	}

	s.mu.Lock()
	s.state = StateDisconnected
	s.tunnel = nil
	s.connectedAt = time.Time{}
	s.mu.Unlock()

	logger.Connection("VPN disconnected")
	s.broadcastStatus()

	return nil
}

// disconnectInternal performs the actual disconnection cleanup.
func (s *Service) disconnectInternal() {
	// Stop domain resolver
	s.routing.StopDomainResolver()

	// Remove all routes
	logger.Info("Removing VPN routes...")
	s.routing.RemoveAllRoutes()

	cfg := s.configManager.Get()

	// Remove VPN server route
	if s.tunnel != nil {
		s.routing.RemoveVPNServerRoute(s.tunnel.ServerIP())
	}

	// Reset DNS
	logger.Info("Resetting DNS configuration...")
	s.dns.Reset()

	// Disable kill switch
	if cfg.KillSwitch.Enabled {
		logger.Info("Disabling kill switch...")
		s.killSwitch.Disable()
	}

	// Stop tunnel
	if s.tunnel != nil {
		logger.Info("Stopping tunnel...")
		s.tunnel.Stop()
	}
}

// cleanupOnError performs cleanup after connection error.
func (s *Service) cleanupOnError(cfg *config.Config) {
	if cfg.KillSwitch.Enabled {
		s.killSwitch.Disable()
	}
	if s.tunnel != nil {
		s.tunnel.Stop()
		s.tunnel = nil
	}
}

// monitorTunnel monitors tunnel state changes.
func (s *Service) monitorTunnel() {
	for {
		s.mu.RLock()
		tunnel := s.tunnel
		s.mu.RUnlock()

		if tunnel == nil {
			return
		}

		for change := range tunnel.StateChanges() {
			switch change.State {
			case protocols.StateDisconnected:
				s.mu.Lock()
				if s.state == StateConnected {
					s.state = StateDisconnected
				}
				s.mu.Unlock()
				s.broadcastStatus()

			case protocols.StateError:
				s.setError(change.Error)

			case protocols.StateReconnecting:
				s.mu.Lock()
				s.state = StateConnecting
				s.mu.Unlock()
				s.broadcastStatus()

			case protocols.StateConnected:
				s.mu.Lock()
				s.state = StateConnected
				if s.connectedAt.IsZero() {
					s.connectedAt = time.Now()
				}
				s.mu.Unlock()
				s.broadcastStatus()
			}
		}

		// Channel was closed. Check if tunnel reconnected (new channel exists).
		s.mu.RLock()
		sameTunnel := s.tunnel == tunnel
		s.mu.RUnlock()

		if !sameTunnel || s.tunnel == nil {
			return // tunnel was replaced or removed — exit
		}

		// Same tunnel object but channel was closed and reopened (Reconnect).
		// Loop back to consume the new channel.
		logger.Info("Monitor: tunnel channel closed, re-attaching to new channel")
	}
}
