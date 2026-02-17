//go:build darwin

package connmon

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
)

func getSystemConnections() ([]Connection, error) {
	out, err := exec.Command("netstat", "-anp", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat failed: %w", err)
	}

	var conns []Connection
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[0] != "tcp4" {
			continue
		}

		local, err := parseDarwinAddrPort(fields[3])
		if err != nil {
			continue
		}
		remote, err := parseDarwinAddrPort(fields[4])
		if err != nil {
			continue
		}

		state := parseDarwinState(fields[5])
		if state == StateListen || state == StateClosed {
			continue
		}

		conns = append(conns, Connection{
			Protocol:   "TCP",
			LocalAddr:  local,
			RemoteAddr: remote,
			State:      state,
		})
	}

	return conns, nil
}

func parseDarwinAddrPort(s string) (netip.AddrPort, error) {
	lastDot := strings.LastIndex(s, ".")
	if lastDot < 0 {
		return netip.AddrPort{}, fmt.Errorf("invalid addr: %s", s)
	}
	ipStr := s[:lastDot]
	portStr := s[lastDot+1:]

	if ipStr == "*" {
		ipStr = "0.0.0.0"
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		if portStr == "*" {
			port = 0
		} else {
			return netip.AddrPort{}, fmt.Errorf("invalid port: %s", portStr)
		}
	}

	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.AddrPort{}, err
	}

	return netip.AddrPortFrom(ip, uint16(port)), nil
}

func parseDarwinState(s string) ConnState {
	switch strings.ToUpper(s) {
	case "ESTABLISHED":
		return StateEstablished
	case "SYN_SENT":
		return StateSynSent
	case "SYN_RECEIVED", "SYN_RECV":
		return StateSynReceived
	case "FIN_WAIT_1":
		return StateFinWait1
	case "FIN_WAIT_2":
		return StateFinWait2
	case "TIME_WAIT":
		return StateTimeWait
	case "CLOSE_WAIT":
		return StateCloseWait
	case "LAST_ACK":
		return StateLastAck
	case "CLOSING":
		return StateClosing
	case "LISTEN":
		return StateListen
	case "CLOSED":
		return StateClosed
	default:
		return StateClosed
	}
}
