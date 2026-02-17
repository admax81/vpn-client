//go:build darwin

package main

import (
	"os"
	"strings"
)

func init() {
	// When launched from Finder / launchd the PATH is minimal
	// (/usr/bin:/bin:/usr/sbin:/sbin) and does not include
	// Homebrew or MacPorts directories where openvpn, wg, etc. live.
	extraPaths := []string{
		"/opt/homebrew/bin", // Homebrew on Apple Silicon
		"/opt/homebrew/sbin",
		"/usr/local/bin", // Homebrew on Intel
		"/usr/local/sbin",
		"/opt/local/bin", // MacPorts
		"/opt/local/sbin",
	}

	current := os.Getenv("PATH")
	parts := strings.Split(current, ":")
	existing := make(map[string]bool, len(parts))
	for _, p := range parts {
		existing[p] = true
	}

	var toAdd []string
	for _, p := range extraPaths {
		if !existing[p] {
			toAdd = append(toAdd, p)
		}
	}

	if len(toAdd) > 0 {
		os.Setenv("PATH", current+":"+strings.Join(toAdd, ":"))
	}
}
