//go:build windows

// Package ui provides Windows GUI for connection monitoring.
// This file is deprecated - code split into multiple files:
// - connmon_models_windows.go: Table models (routing, connections, journal)
// - connmon_main_windows.go: ShowConnMonWindow, globals, ticker
// - connmon_refresh_windows.go: cmRefresh function
// - connmon_routes_windows.go: Route management (add/delete/refresh)
package ui
