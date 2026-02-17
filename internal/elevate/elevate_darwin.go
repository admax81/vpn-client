//go:build darwin

package elevate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// IsAdmin returns true if the current process is running as root.
func IsAdmin() bool {
	return os.Geteuid() == 0
}

// RunAsAdmin re-launches the current executable with root privileges.
// On macOS uses osascript to show a native authorization dialog, then falls back to sudo.
func RunAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks so osascript gets the real path
	resolved, err := resolveExecutable(exe)
	if err == nil {
		exe = resolved
	}

	args := os.Args[1:]

	// Build the shell command with proper quoting
	cmdParts := []string{quoted(exe)}
	for _, a := range args {
		cmdParts = append(cmdParts, quoted(a))
	}
	shellCmd := strings.Join(cmdParts, " ")

	// Try osascript (native macOS authorization dialog)
	if osascriptPath, err := exec.LookPath("osascript"); err == nil {
		script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapeAppleScript(shellCmd))
		cmd := exec.Command(osascriptPath, "-e", script)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			os.Exit(0)
		}
	}

	// Fallback: re-exec with sudo (works only from terminal)
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("osascript and sudo not available; please run as root")
	}

	sudoArgs := append([]string{"sudo", exe}, args...)
	return syscall.Exec(sudoPath, sudoArgs, os.Environ())
}

// resolveExecutable resolves symlinks in the executable path.
func resolveExecutable(exe string) (string, error) {
	return filepath.EvalSymlinks(exe)
}

// quoted wraps a string in single quotes for shell usage.
func quoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// escapeAppleScript escapes a string for use inside an AppleScript double-quoted string.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
