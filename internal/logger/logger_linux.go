//go:build linux

package logger

import (
	"os"
	"path/filepath"
)

// getLogDir returns the log directory next to the executable.
func getLogDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
