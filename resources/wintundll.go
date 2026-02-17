//go:build windows

// Package wintundll embeds wintun.dll and extracts it next to the executable at runtime.
package wintundll

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed wintun.dll
var wintunDLL []byte

// Ensure extracts wintun.dll next to the running executable if it's missing or outdated.
func Ensure() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	dllPath := filepath.Join(filepath.Dir(exePath), "wintun.dll")

	// Check if DLL already exists with correct size
	if info, err := os.Stat(dllPath); err == nil && info.Size() == int64(len(wintunDLL)) {
		return nil // already present and correct size
	}

	// Write embedded DLL to disk
	if err := os.WriteFile(dllPath, wintunDLL, 0644); err != nil {
		return fmt.Errorf("failed to write wintun.dll: %w", err)
	}

	return nil
}
