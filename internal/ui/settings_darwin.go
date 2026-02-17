//go:build darwin

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	vpnconfig "github.com/user/vpn-client/internal/config"
	"github.com/user/vpn-client/internal/logger"
)

// ShowSettingsWindow opens the config file in the user's editor on macOS.
func ShowSettingsWindow() {
	configPath := vpnconfig.GetConfigPath()

	// Ensure config file exists with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := loadAppConfig()
		if err := saveAppConfig(cfg); err != nil {
			logger.Error("Failed to create default config: %v", err)
		}
	}

	// Try $EDITOR first, then macOS open command
	editor := os.Getenv("EDITOR")
	if editor != "" {
		if err := exec.Command(editor, configPath).Start(); err == nil {
			return
		}
	}
	if err := exec.Command("open", "-t", configPath).Start(); err != nil {
		logger.Error("Failed to open config file: %v", err)
		fmt.Printf("Edit config manually: %s\n", configPath)
	}
}

func openLogFile() {
	logPath := logger.GetLogPath()
	if logPath == "" {
		home, _ := os.UserHomeDir()
		logPath = filepath.Join(home, "Library", "Logs", "VPNClient", "vpn.log")
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		dir := filepath.Dir(logPath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(logPath, []byte("VPN Client Log\n"), 0644)
	}
	exec.Command("open", logPath).Start()
}
