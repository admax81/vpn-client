//go:build darwin

package ui

import "github.com/user/vpn-client/internal/core"

// ShowConnectionWindow on macOS falls back to opening settings/config.
func ShowConnectionWindow() {
	ShowSettingsWindow()
}

// RefreshConnectionWindow is a no-op on macOS (no GUI connection window).
func RefreshConnectionWindow(_ *core.StatusPayload) {}
