//go:build !windows

// Package wintundll is a no-op on non-Windows platforms.
package wintundll

// Ensure is a no-op on non-Windows platforms.
func Ensure() error {
	return nil
}
