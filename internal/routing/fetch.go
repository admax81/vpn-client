// Package routing - routing file management (local and SSH-based).
package routing

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// RemoteRoutes holds routes fetched from the routing file.
type RemoteRoutes struct {
	IPs     []string // CIDR or plain IP entries
	Domains []string // domain entries
}

// LocalRoutesFilePath returns the path to routes.txt next to the executable.
func LocalRoutesFilePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to determine executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "routes.txt"), nil
}

// ReadLocalRoutesFile reads and parses the local routes.txt file.
func ReadLocalRoutesFile() (*RemoteRoutes, error) {
	path, err := LocalRoutesFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RemoteRoutes{}, nil
		}
		return nil, fmt.Errorf("failed to read routes file %s: %w", path, err)
	}
	return parseRoutingFile(string(data))
}

// WriteLocalRoute appends a route entry to the local routes.txt file (idempotent).
func WriteLocalRoute(entry string) error {
	path, err := LocalRoutesFilePath()
	if err != nil {
		return err
	}
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return fmt.Errorf("route entry is empty")
	}

	// Check if entry already exists
	existing, _ := os.ReadFile(path)
	scanner := bufio.NewScanner(strings.NewReader(string(existing)))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == entry {
			return nil // already present
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open routes file for writing: %w", err)
	}
	defer f.Close()

	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		f.WriteString("\n")
	}
	_, err = f.WriteString(entry + "\n")
	return err
}

// DeleteLocalRoute removes a route entry from the local routes.txt file.
func DeleteLocalRoute(entry string) error {
	path, err := LocalRoutesFilePath()
	if err != nil {
		return err
	}
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return fmt.Errorf("route entry is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read routes file: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != entry {
			lines = append(lines, line)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// FetchRoutesFromSSH connects to the SSH server and reads the routing file,
// returning parsed IP and domain routes.
func FetchRoutesFromSSH(host string, port int, user, keyPath, password, remotePath string) (*RemoteRoutes, error) {
	if remotePath == "" {
		return nil, fmt.Errorf("remote routing file path is empty")
	}

	cfg, err := buildSSHClientConfig(user, keyPath, password)
	if err != nil {
		return nil, fmt.Errorf("failed to build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.Output(fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		return nil, fmt.Errorf("failed to read remote routing file %s: %w", remotePath, err)
	}

	return parseRoutingFile(string(output))
}

// buildSSHClientConfig creates an ssh.ClientConfig from credentials.
func buildSSHClientConfig(user, keyPath, password string) (*ssh.ClientConfig, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	if keyPath != "" {
		// Remove surrounding quotes if present
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

		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else if password != "" {
		cfg.Auth = []ssh.AuthMethod{ssh.Password(password)}
	} else {
		return nil, fmt.Errorf("no SSH authentication method (key_path or password required)")
	}

	return cfg, nil
}

// parseRoutingFile parses lines from the routing file.
//
// Supported line formats:
//
//	# comment
//	10.0.0.0/8          — IP or CIDR  → added to IPs
//	192.168.1.1         — plain IP    → added to IPs
//	internal.corp.com   — domain      → added to Domains
//	*.example.com       — wildcard    → added to Domains
//
// Empty lines and lines starting with '#' are skipped.
func parseRoutingFile(data string) (*RemoteRoutes, error) {
	routes := &RemoteRoutes{}
	scanner := bufio.NewScanner(strings.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Take only the first token (ignore inline comments after spaces)
		if idx := strings.IndexAny(line, " \t"); idx > 0 {
			line = line[:idx]
		}

		if looksLikeIP(line) {
			routes.IPs = append(routes.IPs, line)
		} else {
			routes.Domains = append(routes.Domains, line)
		}
	}

	return routes, scanner.Err()
}

// WriteRouteToSSH appends a route entry to the remote routing file via SSH.
func WriteRouteToSSH(host string, port int, user, keyPath, password, remotePath, entry string) error {
	if remotePath == "" {
		return fmt.Errorf("remote routing file path is empty")
	}
	if entry == "" {
		return fmt.Errorf("route entry is empty")
	}

	cfg, err := buildSSHClientConfig(user, keyPath, password)
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Append entry on a new line (idempotent: grep checks if already present)
	cmd := fmt.Sprintf(`grep -qxF '%s' %s 2>/dev/null || echo '%s' >> %s`, entry, remotePath, entry, remotePath)
	if _, err := session.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("failed to write route '%s' to remote file: %w", entry, err)
	}

	return nil
}

// DeleteRouteFromSSH removes a route entry from the remote routing file via SSH.
func DeleteRouteFromSSH(host string, port int, user, keyPath, password, remotePath, entry string) error {
	if remotePath == "" {
		return fmt.Errorf("remote routing file path is empty")
	}
	if entry == "" {
		return fmt.Errorf("route entry is empty")
	}

	cfg, err := buildSSHClientConfig(user, keyPath, password)
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Use sed to remove the exact line
	escaped := strings.ReplaceAll(entry, "/", "\\/")
	escaped = strings.ReplaceAll(escaped, ".", "\\.")
	escaped = strings.ReplaceAll(escaped, "*", "\\*")
	cmd := fmt.Sprintf(`sed -i '/^%s$/d' %s`, escaped, remotePath)
	if _, err := session.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("failed to delete route '%s' from remote file: %w", entry, err)
	}

	return nil
}

// CheckRoutingFileWritable tests if the remote routing file is writable.
func CheckRoutingFileWritable(host string, port int, user, keyPath, password, remotePath string) error {
	if remotePath == "" {
		return fmt.Errorf("remote routing file path is empty")
	}

	cfg, err := buildSSHClientConfig(user, keyPath, password)
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Check if file is writable using test command
	cmd := fmt.Sprintf(`test -w %s`, remotePath)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("routing file is not writable")
	}

	return nil
}

// looksLikeIP returns true if s starts with a digit or contains '/' (CIDR) or ':' (IPv6).
func looksLikeIP(s string) bool {
	if s == "" {
		return false
	}
	// CIDR notation
	if strings.Contains(s, "/") {
		return true
	}
	// IPv6
	if strings.Contains(s, ":") {
		return true
	}
	// Starts with digit → likely IPv4
	return s[0] >= '0' && s[0] <= '9'
}
