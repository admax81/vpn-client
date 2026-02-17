package ssh

import (
	"encoding/base64"
	"fmt"
	"io"

	"github.com/user/vpn-client/internal/logger"
)

// requestTunnel sets up the remote TUN device and starts packet forwarding.
func (t *Tunnel) requestTunnel() error {
	// Set up the remote TUN interface via exec
	// The server needs to allow the user to create TUN devices

	tunNum := 0 // Use tun0 device

	stdin, err := t.session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := t.session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := t.session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Log stderr in background
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				logger.Error("Remote stderr: %s", string(buf[:n]))
				fmt.Printf("SSH STDERR: %s\n", string(buf[:n]))
			}
		}
	}()

	// DO NOT request PTY - it corrupts binary data!

	// Get local and remote IPs for the tunnel
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

	// Use sudo only if user is not root
	isRoot := t.cfg.User == "root"
	sudoPrefix := ""
	if !isRoot {
		sudoPrefix = "sudo "
	}

	// Python script for TUN forwarding - creates TUN device itself
	pythonScript := `
import os, sys, select, fcntl, struct, signal, subprocess, time, errno

def handler(sig, frame):
    sys.exit(0)
signal.signal(signal.SIGTERM, handler)
signal.signal(signal.SIGINT, handler)

TUNSETIFF = 0x400454ca
IFF_TUN = 0x0001
IFF_NO_PI = 0x1000

tun_base = os.environ.get('TUN_NAME', 'tun0')
local_ip = os.environ.get('LOCAL_IP', '10.0.0.1')
remote_ip = os.environ.get('REMOTE_IP', '10.0.0.2')

# Try to find an available TUN device (try up to 10 devices)
tun_fd = None
tun_name = None
last_error = None

for attempt in range(10):
    if attempt == 0:
        tun_name = tun_base
    else:
        # Extract base name and number, then try next number
        base_num = int(tun_base.replace('tun', ''))
        tun_name = 'tun{}'.format(base_num + attempt)
    
    # Cleanup any existing tun device completely
    subprocess.run(['ip', 'link', 'set', tun_name, 'down'], stderr=subprocess.DEVNULL)
    subprocess.run(['ip', 'addr', 'flush', 'dev', tun_name], stderr=subprocess.DEVNULL)
    subprocess.run(['ip', 'tuntap', 'del', 'dev', tun_name, 'mode', 'tun'], stderr=subprocess.DEVNULL)
    time.sleep(0.5)  # Increased wait time for kernel cleanup
    
    try:
        # Open TUN control device and create interface
        tun_fd = os.open('/dev/net/tun', os.O_RDWR)
        ifr = struct.pack('16sH', tun_name.encode(), IFF_TUN | IFF_NO_PI)
        fcntl.ioctl(tun_fd, TUNSETIFF, ifr)
        break  # Success!
    except OSError as e:
        last_error = e
        if tun_fd is not None:
            try:
                os.close(tun_fd)
            except:
                pass
            tun_fd = None
        if e.errno != errno.EBUSY:
            # If it's not a "busy" error, don't retry
            raise
        # Otherwise, try next device number

if tun_fd is None:
    raise OSError(errno.EBUSY, 'All TUN devices busy, last error: {}'.format(last_error))

# Log which device we're using
sys.stderr.write('Using TUN device: {}\n'.format(tun_name))
sys.stderr.flush()

# Configure the interface - flush any existing addresses first
subprocess.run(['ip', 'addr', 'flush', 'dev', tun_name], stderr=subprocess.DEVNULL)
subprocess.run(['ip', 'addr', 'add', local_ip, 'peer', remote_ip, 'dev', tun_name], check=True)
subprocess.run(['ip', 'link', 'set', tun_name, 'up'], check=True)

# Signal that we're ready
os.write(sys.stdout.fileno(), b'READY\n')

stdin_fd = sys.stdin.fileno()
stdout_fd = sys.stdout.fileno()

# Main forwarding loop
while True:
    r, _, _ = select.select([tun_fd, stdin_fd], [], [])
    for fd in r:
        if fd == tun_fd:
            pkt = os.read(tun_fd, 65535)
            if pkt:
                os.write(stdout_fd, struct.pack('>H', len(pkt)) + pkt)
        elif fd == stdin_fd:
            hdr = b''
            while len(hdr) < 2:
                chunk = os.read(stdin_fd, 2 - len(hdr))
                if not chunk:
                    sys.exit(0)
                hdr += chunk
            pkt_len = struct.unpack('>H', hdr)[0]
            pkt = b''
            while len(pkt) < pkt_len:
                chunk = os.read(stdin_fd, pkt_len - len(pkt))
                if not chunk:
                    sys.exit(0)
                pkt += chunk
            os.write(tun_fd, pkt)
`

	// Encode the Python script in base64 to avoid shell escaping issues
	// and to skip Setenv (most servers reject it via AcceptEnv, and some
	// close the channel on rejection, leaving the session in a broken state).
	encodedScript := base64.StdEncoding.EncodeToString([]byte(pythonScript))

	setupCmd := fmt.Sprintf(`
set -e
%[1]ssysctl -q -w net.ipv4.ip_forward=1
%[1]siptables -t nat -C POSTROUTING -s %[4]s -j MASQUERADE 2>/dev/null || %[1]siptables -t nat -A POSTROUTING -s %[4]s -j MASQUERADE
export TUN_NAME=tun%[2]d
export LOCAL_IP=%[3]s
export REMOTE_IP=%[4]s
exec %[1]spython3 -u -c "$(echo '%[5]s' | base64 -d)"
`, sudoPrefix, tunNum, remoteIP, localIP, encodedScript)

	if err := t.session.Start(setupCmd); err != nil {
		return fmt.Errorf("failed to start tunnel command: %w", err)
	}
	logger.Info("Tunnel setup command started on remote server")

	// Wait for READY signal from Python
	readyBuf := make([]byte, 6)
	if _, err := io.ReadFull(stdout, readyBuf); err != nil {
		return fmt.Errorf("failed to read READY signal: %w", err)
	}
	if string(readyBuf) != "READY\n" {
		return fmt.Errorf("unexpected signal from remote: %s", string(readyBuf))
	}
	logger.Info("Remote TUN device ready")
	fmt.Printf("SSH LOG: Remote TUN device ready\n")

	// Store stdin/stdout for packet forwarding
	t.startPacketBridge(stdin, stdout)
	logger.Debug("Packet bridge started")

	return nil
}
