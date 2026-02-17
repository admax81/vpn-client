//go:build linux || darwin

package elevate

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// IsAdmin returns true if the current process is running as root.
func IsAdmin() bool {
	return os.Geteuid() == 0
}

// RunAsAdmin re-launches the current executable with root privileges.
// On Linux tries pkexec, then sudo. On macOS tries osascript, then sudo.
func RunAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	args := append([]string{exe}, os.Args[1:]...)

	// Try pkexec (Linux graphical sudo)
	if path, err := exec.LookPath("pkexec"); err == nil {
		cmd := &exec.Cmd{
			Path:   path,
			Args:   append([]string{"pkexec"}, args...),
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		if err := cmd.Start(); err == nil {
			os.Exit(0)
		}
	}

	// Fallback: re-exec with sudo
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("neither pkexec nor sudo found; please run as root")
	}

	return syscall.Exec(sudoPath, append([]string{"sudo"}, args...), os.Environ())
}
