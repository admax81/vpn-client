//go:build !windows

package logger

import (
	"os"
	"syscall"
)

// redirectStderr redirects stderr to the log file so panics are captured.
func redirectStderr(f *os.File) {
	syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
}
