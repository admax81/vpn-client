//go:build darwin

package ui

import "github.com/user/vpn-client/internal/core"

// ShowConnectionWindow is a no-op on macOS.
// The systray menu itself serves as the connection UI
// (connect / disconnect / status).
func ShowConnectionWindow() {}

// RefreshConnectionWindow is a no-op on macOS (no GUI connection window).
func RefreshConnectionWindow(_ *core.StatusPayload) {}
