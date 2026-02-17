//go:build linux

package config

import (
	"os"
	"path/filepath"
)

// GetConfigPath returns the configuration path next to the executable.
func GetConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}

// defaultInterfaceName returns the default TUN interface name for Linux.
func defaultInterfaceName() string {
	return "VPNClient"
}
