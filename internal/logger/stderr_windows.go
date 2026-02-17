//go:build windows

package logger

import (
	"os"

	"golang.org/x/sys/windows"
)

// redirectStderr redirects stderr to the log file so panics are captured.
func redirectStderr(f *os.File) {
	handle := windows.Handle(f.Fd())
	// STD_ERROR_HANDLE = -12
	proc := windows.NewLazyDLL("kernel32.dll").NewProc("SetStdHandle")
	proc.Call(uintptr(0xFFFFFFF4), uintptr(handle)) //nolint:errcheck
	os.Stderr = f
}
