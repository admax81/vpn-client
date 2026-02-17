// Package core provides the main VPN service logic.
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/dns"
	"github.com/user/vpn-client/internal/killswitch"
	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/protocols"
	"github.com/user/vpn-client/internal/routing"
)

// State represents the VPN service state.
type State string

const (
	StateDisconnected  State = "disconnected"
	StateConnecting    State = "connecting"
	StateConnected     State = "connected"
	StateDisconnecting State = "disconnecting"
	StateError         State = "error"
)

// StatusPayload represents the VPN status for UI updates.
type StatusPayload struct {
	State         string
	Protocol      string
	ServerAddress string
	LocalIP       string
	ConnectedAt   time.Time
	BytesSent     uint64
	BytesReceived uint64
	Error         string
}

// StatusListener is a callback invoked when VPN status changes.
type StatusListener func(status *StatusPayload)

// Service is the main VPN service.
type Service struct {
	mu             sync.RWMutex
	state          State
	configManager  *config.Manager
	tunnel         protocols.Tunnel
	routing        *routing.Manager
	dns            *dns.Manager
	killSwitch     *killswitch.KillSwitch
	ctx            context.Context
	cancel         context.CancelFunc
	connectedAt    time.Time
	lastError      error
	wasConnected   bool // For resume after sleep
	statusListener StatusListener
}

// NewService creates a new VPN service.
func NewService(configPath string) (*Service, error) {
	// Initialize logger
	logger.Init()
	logger.Info("VPN Service initializing...")

	configManager := config.NewManager(configPath)
	if err := configManager.Load(); err != nil {
		logger.Error("Failed to load config: " + err.Error())
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger.Info("Configuration loaded successfully")

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		state:         StateDisconnected,
		configManager: configManager,
		routing:       routing.NewManager(),
		dns:           dns.NewManager(),
		killSwitch:    killswitch.New(),
		ctx:           ctx,
		cancel:        cancel,
	}

	logger.Info("VPN Service initialized")
	return s, nil
}

// SetStatusListener sets a callback that will be called on every status change.
func (s *Service) SetStatusListener(listener StatusListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusListener = listener
}

// Start starts the VPN service.
func (s *Service) Start() error {
	logger.Info("Starting VPN service...")

	// Auto-connect if configured
	cfg := s.configManager.Get()
	if cfg.Autostart {
		logger.Info("Auto-connect enabled, will connect in 2 seconds...")
		go func() {
			defer logger.Recover("autoConnect")
			time.Sleep(2 * time.Second) // Wait for system to settle
			s.Connect()
		}()
	}

	logger.Info("VPN service started successfully")
	return nil
}

// Stop stops the VPN service.
func (s *Service) Stop() error {
	logger.Info("Stopping VPN service...")
	s.cancel()

	// Disconnect if connected
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	if state == StateConnected || state == StateConnecting {
		s.Disconnect()
	}

	logger.Info("VPN service stopped")
	logger.Close()
	return nil
}
