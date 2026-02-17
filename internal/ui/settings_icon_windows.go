//go:build windows

package ui

import (
	"os"

	"github.com/lxn/walk"
)

// createWindowIcon generates a walk.Icon from the shield ICO data.
func createWindowIcon() *walk.Icon {
	icoData := GenerateMaskIcon(85, 187, 187)

	tmpFile, err := os.CreateTemp("", "vpn-icon-*.ico")
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(icoData); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil
	}
	tmpFile.Close()

	icon, err := walk.NewIconFromFile(tmpPath)
	os.Remove(tmpPath)
	if err != nil {
		return nil
	}

	return icon
}
