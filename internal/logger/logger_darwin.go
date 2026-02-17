//go:build darwin

package logger

import (
	"os"
	"path/filepath"
)

// getLogDir returns the log directory.
// Uses ~/Library/Application Support/VPN Client/ so logs are writable
// even when running from a signed/notarized .app bundle.
func getLogDir() string {
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, "Library", "Application Support", "VPN Client")
		_ = os.MkdirAll(dir, 0755)
		return dir
	}

	// Fallback: next to executable
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
