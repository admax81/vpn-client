//go:build windows

// Package ui provides Windows GUI for connection status.
// This file is deprecated - code split into multiple files:
// - connection_main_windows.go: ShowConnectionWindow, globals, ticker
// - connection_refresh_windows.go: RefreshConnectionWindow, UI refresh, formatters
// - connection_dot_windows.go: Status indicator dot rendering
package ui
