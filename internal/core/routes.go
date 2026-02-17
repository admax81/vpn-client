package core

import (
	"fmt"
	"os"

	"github.com/user/vpn-client/internal/logger"
	"github.com/user/vpn-client/internal/routing"
)

// FetchRemoteRoutes reads routes from the local routes.txt file.
func (s *Service) FetchRemoteRoutes() (*routing.RemoteRoutes, error) {
	return routing.ReadLocalRoutesFile()
}

// AddRemoteRoute adds a route entry to the local routes file and, if connected,
// also adds it to the local routing table live.
func (s *Service) AddRemoteRoute(value, routeType string) error {
	if err := routing.WriteLocalRoute(value); err != nil {
		return fmt.Errorf("failed to write route to local file: %w", err)
	}
	logger.Info("Route added to local routes file: " + value)

	// If connected, also add to local routing table
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == StateConnected {
		if routeType == "IP/CIDR" {
			if err := s.routing.AddRoute(value, "remote"); err != nil {
				logger.Warning("Failed to add local route for " + value + ": " + err.Error())
			} else {
				logger.Info("Local route added: " + value)
			}
		} else {
			// Domain — resolve and add
			if err := s.routing.AddDomainRoute(value); err != nil {
				logger.Warning("Failed to add domain route for " + value + ": " + err.Error())
			} else {
				logger.Info("Local domain route added: " + value)
			}
		}
	}

	return nil
}

// RemoveRemoteRoute removes a route entry from the local routes file and, if connected,
// removes it from the local routing table live.
func (s *Service) RemoveRemoteRoute(value, routeType string) error {
	if err := routing.DeleteLocalRoute(value); err != nil {
		return fmt.Errorf("failed to delete route from local file: %w", err)
	}
	logger.Info("Route removed from local routes file: " + value)

	// If connected, also remove from local routing table
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()

	if state == StateConnected {
		if routeType == "IP/CIDR" {
			if err := s.routing.RemoveRoute(value); err != nil {
				logger.Warning("Failed to remove local route for " + value + ": " + err.Error())
			} else {
				logger.Info("Local route removed: " + value)
			}
		}
		if routeType == "Domain" {
			logger.Info("Domain removed from routes file; local resolved routes will expire on reconnect: " + value)
		}
	}

	return nil
}

// GetRoutingManager returns the routing manager (for read-only queries from the UI).
func (s *Service) GetRoutingManager() *routing.Manager {
	return s.routing
}

// CheckRoutingFileWritable checks if the local routes file is writable.
func (s *Service) CheckRoutingFileWritable() error {
	path, err := routing.LocalRoutesFilePath()
	if err != nil {
		return err
	}
	// Try opening for append to check write permission
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("routing file is not writable: %w", err)
	}
	f.Close()
	return nil
}
