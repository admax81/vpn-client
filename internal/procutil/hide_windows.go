//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// HideWindow configures the command to run without showing a console window.
func HideWindow(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd
}
