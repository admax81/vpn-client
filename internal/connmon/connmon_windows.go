//go:build windows

package connmon

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"syscall"
	"unsafe"
)

var (
	modIphlpapi             = syscall.NewLazyDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modIphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modIphlpapi.NewProc("GetExtendedUdpTable")
)

const (
	tcpTableOwnerPidAll = 5
	udpTableOwnerPid    = 1
	afInet              = 2
)

// MIB_TCPROW_OWNER_PID
type tcpRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPid  uint32
}

// MIB_UDPROW_OWNER_PID
type udpRowOwnerPID struct {
	LocalAddr uint32
	LocalPort uint32
	OwningPid uint32
}

func getSystemConnections() ([]Connection, error) {
	var conns []Connection

	tcp, err := getTCPConnections()
	if err == nil {
		conns = append(conns, tcp...)
	}

	udp, err := getUDPConnections()
	if err == nil {
		conns = append(conns, udp...)
	}

	if len(conns) == 0 && err != nil {
		return nil, fmt.Errorf("failed to get connections: %w", err)
	}

	return conns, nil
}

func getTCPConnections() ([]Connection, error) {
	var size uint32
	ret, _, _ := procGetExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0,
		afInet, tcpTableOwnerPidAll, 0)
	if ret != 0 && ret != 122 { // 122 = ERROR_INSUFFICIENT_BUFFER
		return nil, fmt.Errorf("GetExtendedTcpTable size query failed: %d", ret)
	}

	buf := make([]byte, size)
	ret, _, _ = procGetExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)), 0, afInet, tcpTableOwnerPidAll, 0)
	if ret != 0 {
		return nil, fmt.Errorf("GetExtendedTcpTable failed: %d", ret)
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	var conns []Connection

	rowSize := unsafe.Sizeof(tcpRowOwnerPID{})
	for i := uint32(0); i < numEntries; i++ {
		offset := 4 + uintptr(i)*rowSize
		row := (*tcpRowOwnerPID)(unsafe.Pointer(&buf[offset]))

		state := ConnState(row.State)
		// Skip LISTEN and CLOSED states for display
		if state == StateListen || state == StateClosed {
			continue
		}

		local := makeAddrPort(row.LocalAddr, row.LocalPort)
		remote := makeAddrPort(row.RemoteAddr, row.RemotePort)

		conns = append(conns, Connection{
			Protocol:   "TCP",
			LocalAddr:  local,
			RemoteAddr: remote,
			State:      state,
			PID:        row.OwningPid,
		})
	}

	return conns, nil
}

func getUDPConnections() ([]Connection, error) {
	var size uint32
	ret, _, _ := procGetExtendedUdpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0,
		afInet, udpTableOwnerPid, 0)
	if ret != 0 && ret != 122 {
		return nil, fmt.Errorf("GetExtendedUdpTable size query failed: %d", ret)
	}

	buf := make([]byte, size)
	ret, _, _ = procGetExtendedUdpTable.Call(uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)), 0, afInet, udpTableOwnerPid, 0)
	if ret != 0 {
		return nil, fmt.Errorf("GetExtendedUdpTable failed: %d", ret)
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	var conns []Connection

	rowSize := unsafe.Sizeof(udpRowOwnerPID{})
	for i := uint32(0); i < numEntries; i++ {
		offset := 4 + uintptr(i)*rowSize
		row := (*udpRowOwnerPID)(unsafe.Pointer(&buf[offset]))

		local := makeAddrPort(row.LocalAddr, row.LocalPort)
		// UDP is connectionless, remote is unknown
		remote := netip.AddrPortFrom(netip.IPv4Unspecified(), 0)

		conns = append(conns, Connection{
			Protocol:   "UDP",
			LocalAddr:  local,
			RemoteAddr: remote,
			PID:        row.OwningPid,
		})
	}

	return conns, nil
}

// resolveProcessNames populates ProcessName for each connection by looking up PID.
func resolveProcessNames(conns []Connection) {
	// Cache PID -> name to avoid repeated lookups in the same poll cycle
	cache := make(map[uint32]string)
	for i := range conns {
		pid := conns[i].PID
		if pid == 0 {
			continue
		}
		if name, ok := cache[pid]; ok {
			conns[i].ProcessName = name
			continue
		}
		name := getProcessName(pid)
		cache[pid] = name
		conns[i].ProcessName = name
	}
}

// getProcessName returns the executable name for a given PID.
func getProcessName(pid uint32) string {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess := kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW := kernel32.NewProc("QueryFullProcessImageNameW")

	handle, _, _ := procOpenProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return ""
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	var buf [260]uint16
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return ""
	}

	fullPath := syscall.UTF16ToString(buf[:size])
	// Extract just the filename
	for i := len(fullPath) - 1; i >= 0; i-- {
		if fullPath[i] == '\\' || fullPath[i] == '/' {
			return fullPath[i+1:]
		}
	}
	return fullPath
}

func makeAddrPort(rawAddr uint32, rawPort uint32) netip.AddrPort {
	ip := netip.AddrFrom4([4]byte{
		byte(rawAddr),
		byte(rawAddr >> 8),
		byte(rawAddr >> 16),
		byte(rawAddr >> 24),
	})
	// Port is in network byte order (big-endian) stored in the low 16 bits
	port := uint16((rawPort>>8)&0xFF | (rawPort<<8)&0xFF00)
	return netip.AddrPortFrom(ip, port)
}
