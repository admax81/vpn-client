package ssh

import (
	"io"
)

// startPacketBridge bridges packets between local TUN and remote SSH tunnel.
func (t *Tunnel) startPacketBridge(stdin io.WriteCloser, stdout io.Reader) {
	// Read from TUN, write length-prefixed packets to SSH
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		buf := make([]byte, 65535)
		lenBuf := make([]byte, 2)
		for {
			select {
			case <-t.stopCh:
				return
			default:
			}

			n, err := t.adapter.Read(buf, 0)
			if err != nil {
				continue
			}

			if n > 0 {
				// Write length prefix (big-endian uint16)
				lenBuf[0] = byte(n >> 8)
				lenBuf[1] = byte(n)
				if _, err := stdin.Write(lenBuf); err != nil {
					return
				}
				if _, err := stdin.Write(buf[:n]); err != nil {
					return
				}
				t.Statistics.BytesSent += uint64(n)
				t.Statistics.PacketsSent++
			}
		}
	}()

	// Read length-prefixed packets from SSH, write to TUN
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		lenBuf := make([]byte, 2)
		pktBuf := make([]byte, 65535)
		for {
			select {
			case <-t.stopCh:
				return
			default:
			}

			// Read length prefix
			if _, err := io.ReadFull(stdout, lenBuf); err != nil {
				return
			}
			pktLen := int(lenBuf[0])<<8 | int(lenBuf[1])
			if pktLen > len(pktBuf) || pktLen == 0 {
				continue
			}

			// Read packet data
			if _, err := io.ReadFull(stdout, pktBuf[:pktLen]); err != nil {
				return
			}

			// Write to TUN
			if _, err := t.adapter.Write(pktBuf[:pktLen], 0); err != nil {
				continue
			}
			t.Statistics.BytesReceived += uint64(pktLen)
			t.Statistics.PacketsRecv++
		}
	}()
}

// startForwarding sets up packet forwarding.
func (t *Tunnel) startForwarding() error {
	// Set up routing on local machine to send traffic to remote via the TUN
	// This is handled by the routing module, but we need to set up the point-to-point link

	// Get local and remote IPs
	localIP := t.cfg.LocalTunAddr
	if localIP == "" {
		localIP = "10.0.0.2"
	}
	if idx := indexOf(localIP, '/'); idx > 0 {
		localIP = localIP[:idx]
	}

	remoteIP := t.cfg.RemoteTunAddr
	if remoteIP == "" {
		remoteIP = "10.0.0.1"
	}
	if idx := indexOf(remoteIP, '/'); idx > 0 {
		remoteIP = remoteIP[:idx]
	}

	return nil
}
