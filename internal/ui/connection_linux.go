//go:build linux

package ui

import "github.com/user/vpn-client/internal/core"

// ShowConnectionWindow on Linux falls back to opening settings/config.
func ShowConnectionWindow() {
	ShowSettingsWindow()
}

// RefreshConnectionWindow is a no-op on Linux (no GUI connection window).
func RefreshConnectionWindow(_ *core.StatusPayload) {}
