//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

// GetConfigPath returns the configuration path.
// When running as a .app bundle the config is stored in
// ~/Library/Application Support/VPN Client/ so that it
// survives app updates and is writable (the bundle itself is read-only).
// Falls back to the directory next to the executable for CLI usage.
func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, "Library", "Application Support", "VPN Client")
		// Ensure the directory exists
		_ = os.MkdirAll(dir, 0755)
		return filepath.Join(dir, "config.yaml")
	}

	// Fallback: next to executable (CLI / non-bundle usage)
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}

// defaultInterfaceName returns the default TUN interface name for macOS.
// macOS requires utun[0-9]* — using "utun" lets the OS auto-assign a number.
func defaultInterfaceName() string {
	return "utun"
}
