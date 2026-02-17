//go:build linux

package connmon

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

func getSystemConnections() ([]Connection, error) {
	var conns []Connection

	tcp, err := parseProc("/proc/net/tcp")
	if err == nil {
		for i := range tcp {
			tcp[i].Protocol = "TCP"
		}
		conns = append(conns, tcp...)
	}

	udp, err := parseProc("/proc/net/udp")
	if err == nil {
		for i := range udp {
			udp[i].Protocol = "UDP"
		}
		conns = append(conns, udp...)
	}

	if len(conns) == 0 && err != nil {
		return nil, fmt.Errorf("failed to read /proc/net: %w", err)
	}

	return conns, nil
}

func parseProc(path string) ([]Connection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conns []Connection
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		local, err := parseHexAddrPort(fields[1])
		if err != nil {
			continue
		}
		remote, err := parseHexAddrPort(fields[2])
		if err != nil {
			continue
		}

		stateVal, _ := strconv.ParseUint(fields[3], 16, 32)
		state := ConnState(stateVal)

		if state == StateListen || state == StateClosed {
			continue
		}

		// UID is in field 7, inode in field 9
		conns = append(conns, Connection{
			LocalAddr:  local,
			RemoteAddr: remote,
			State:      state,
		})
	}

	return conns, nil
}

func parseHexAddrPort(s string) (netip.AddrPort, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return netip.AddrPort{}, fmt.Errorf("invalid format: %s", s)
	}

	ipBytes, err := hex.DecodeString(parts[0])
	if err != nil || len(ipBytes) != 4 {
		return netip.AddrPort{}, fmt.Errorf("invalid ip: %s", parts[0])
	}

	// Linux stores IP in little-endian
	ip := netip.AddrFrom4([4]byte{ipBytes[3], ipBytes[2], ipBytes[1], ipBytes[0]})

	portVal, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid port: %s", parts[1])
	}

	return netip.AddrPortFrom(ip, uint16(portVal)), nil
}
