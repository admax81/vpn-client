// VPN Client - Combined Service and UI
package main

import (
	"fmt"
	"log"

	"github.com/user/vpn-client/internal/elevate"
	"github.com/user/vpn-client/internal/ui"
	wintundll "github.com/user/vpn-client/resources"
)

func main() {
	fmt.Println("VPN Client starting...")

	// VPN requires admin/root for TUN, routing, DNS and firewall
	if !elevate.IsAdmin() {
		fmt.Println("Not running as administrator, requesting elevation...")
		if err := elevate.RunAsAdmin(); err != nil {
			log.Fatalf("Failed to elevate privileges: %v\nPlease run as administrator.", err)
		}
		return // elevated process was launched
	}

	// Ensure wintun.dll is available (Windows-only, no-op elsewhere)
	if err := wintundll.Ensure(); err != nil {
		log.Fatalf("Failed to prepare TUN driver: %v", err)
	}

	ui.Run()
}
