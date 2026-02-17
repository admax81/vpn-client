//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

// GetConfigPath returns the configuration path next to the executable.
// For .app bundles this resolves to Contents/MacOS/ inside the bundle.
func GetConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}
