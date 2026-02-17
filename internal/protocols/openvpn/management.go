package openvpn

import (
	"bufio"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/user/vpn-client/internal/protocols"
)

func (t *Tunnel) connectManagement() error {
	var err error
	for i := 0; i < 10; i++ {
		t.mgmtConn, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", managementPort), time.Second)
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("failed to connect to management interface: %w", err)
}

func (t *Tunnel) sendCommand(cmd string) error {
	if t.mgmtConn == nil {
		return fmt.Errorf("management not connected")
	}

	t.mgmtConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := t.mgmtConn.Write([]byte(cmd + "\n"))
	return err
}

func (t *Tunnel) monitorManagement() {
	scanner := bufio.NewScanner(t.mgmtConn)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse management interface messages
		if strings.HasPrefix(line, ">STATE:") {
			t.handleStateChange(line)
		} else if strings.HasPrefix(line, ">BYTECOUNT:") {
			t.handleByteCount(line)
		} else if strings.HasPrefix(line, ">PASSWORD:") {
			t.handlePasswordRequest(line)
		} else if strings.HasPrefix(line, ">INFO:") {
			t.handleInfo(line)
		}

		select {
		case <-t.ctx.Done():
			return
		default:
		}
	}
}

func (t *Tunnel) handleStateChange(line string) {
	// Format: >STATE:timestamp,state,description,local_ip,remote_ip
	parts := strings.Split(strings.TrimPrefix(line, ">STATE:"), ",")
	if len(parts) < 2 {
		return
	}

	state := parts[1]

	switch state {
	case "CONNECTED":
		if len(parts) >= 4 {
			if ip, err := netip.ParseAddr(parts[3]); err == nil {
				t.LocalIPAddr = ip
			}
		}
		select {
		case <-t.connected:
		default:
			close(t.connected)
		}
		t.SetState(protocols.StateConnected, "Connected", nil)

	case "RECONNECTING":
		t.SetState(protocols.StateReconnecting, "Reconnecting", nil)

	case "EXITING":
		t.SetState(protocols.StateDisconnecting, "Exiting", nil)

	case "CONNECTING", "WAIT", "AUTH", "GET_CONFIG":
		t.SetState(protocols.StateConnecting, state, nil)
	}
}

func (t *Tunnel) handleByteCount(line string) {
	// Format: >BYTECOUNT:bytes_in,bytes_out
	parts := strings.Split(strings.TrimPrefix(line, ">BYTECOUNT:"), ",")
	if len(parts) >= 2 {
		bytesIn, _ := strconv.ParseUint(parts[0], 10, 64)
		bytesOut, _ := strconv.ParseUint(parts[1], 10, 64)
		t.Statistics.BytesReceived = bytesIn
		t.Statistics.BytesSent = bytesOut
	}
}

func (t *Tunnel) handlePasswordRequest(line string) {
	// Format: >PASSWORD:Need 'Auth' username/password
	if strings.Contains(line, "Auth") && t.cfg.AuthUser != "" {
		// Send username and password
		t.sendCommand(fmt.Sprintf("username 'Auth' %s", t.cfg.AuthUser))
		t.sendCommand(fmt.Sprintf("password 'Auth' %s", t.cfg.AuthPass))
	}
}

func (t *Tunnel) handleInfo(line string) {
	// Log info messages
	// Could be used for additional state tracking
}
