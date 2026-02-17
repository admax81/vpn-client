//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// GetConfigPath returns the configuration path next to the executable.
func GetConfigPath() string {
	return configPathNextToExe()
}

func configPathNextToExe() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml" // fallback: current directory
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}

// defaultInterfaceName returns the default TUN interface name for Windows.
func defaultInterfaceName() string {
	return "VPNClient"
}
