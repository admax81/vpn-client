package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Manager handles configuration operations.
type Manager struct {
	mu         sync.RWMutex
	config     *Config
	configPath string
}

// NewManager creates a new configuration manager.
func NewManager(configPath string) *Manager {
	return &Manager{
		configPath: configPath,
	}
}

// Load reads configuration from file.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.config = DefaultConfig()
			return m.saveUnsafe()
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.config = &cfg
	return nil
}

// Save writes configuration to file.
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveUnsafe()
}

func (m *Manager) saveUnsafe() error {
	if m.config == nil {
		return fmt.Errorf("no configuration to save")
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Get returns the current configuration.
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Update updates the configuration.
func (m *Manager) Update(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	m.config = cfg
	m.mu.Unlock()

	return m.Save()
}
