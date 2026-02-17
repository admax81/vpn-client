//go:build windows

package elevate

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsAdmin returns true if the current process has administrator privileges.
func IsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	member, err := windows.Token(0).IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

// RunAsAdmin re-launches the current executable with administrator privileges via UAC.
// If the user accepts the UAC prompt, this function does not return (it exits the current process).
// If the user cancels, it returns an error.
func RunAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	args := strings.Join(os.Args[1:], " ")

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString(args)
	cwd, _ := syscall.UTF16PtrFromString("")

	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")

	ret, _, _ := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(cwd)),
		uintptr(syscall.SW_NORMAL),
	)

	// ShellExecuteW returns > 32 on success
	if ret <= 32 {
		return fmt.Errorf("UAC elevation failed or was cancelled (code %d)", ret)
	}

	// Elevated process launched — exit current (non-admin) process
	os.Exit(0)
	return nil // unreachable
}
