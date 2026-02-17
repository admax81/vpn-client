//go:build linux

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	vpnconfig "github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/logger"
)

// ShowSettingsWindow opens the config file in the user's editor on Linux.
func ShowSettingsWindow() {
	configPath := vpnconfig.GetConfigPath()

	// Ensure config file exists with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := loadAppConfig()
		if err := saveAppConfig(cfg); err != nil {
			logger.Error("Failed to create default config: %v", err)
		}
	}

	// Try editors in order: $EDITOR, xdg-open, nano, vi
	editor := os.Getenv("EDITOR")
	if editor != "" {
		if err := exec.Command(editor, configPath).Start(); err == nil {
			return
		}
	}
	if err := exec.Command("xdg-open", configPath).Start(); err == nil {
		return
	}
	// Terminal fallback
	for _, ed := range []string{"nano", "vi"} {
		if path, err := exec.LookPath(ed); err == nil {
			cmd := exec.Command(path, configPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			return
		}
	}

	logger.Error("No editor found to open config: %s", configPath)
	fmt.Printf("Edit config manually: %s\n", configPath)
}

func openLogFile() {
	logPath := logger.GetLogPath()
	if logPath == "" {
		home, _ := os.UserHomeDir()
		logPath = filepath.Join(home, ".local", "state", "vpn-client", "vpn.log")
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		dir := filepath.Dir(logPath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(logPath, []byte("VPN Client Log\n"), 0644)
	}
	exec.Command("xdg-open", logPath).Start()
}
