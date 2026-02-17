package ssh

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// buildSSHConfig creates SSH client configuration.
func (t *Tunnel) buildSSHConfig() (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            t.cfg.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add proper host key verification
		Timeout:         30 * time.Second,
	}

	// Try key-based auth first if key_path is specified
	if t.cfg.KeyPath != "" {
		// Remove quotes if present
		keyPath := t.cfg.KeyPath
		if len(keyPath) > 2 && keyPath[0] == '"' && keyPath[len(keyPath)-1] == '"' {
			keyPath = keyPath[1 : len(keyPath)-1]
		}

		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key '%s': %w", keyPath, err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	} else if t.cfg.Password != "" {
		// Fall back to password auth
		config.Auth = []ssh.AuthMethod{
			ssh.Password(t.cfg.Password),
		}
	} else {
		return nil, fmt.Errorf("no authentication method specified (key_path or password required)")
	}

	return config, nil
}
