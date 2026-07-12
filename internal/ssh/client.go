// Package ssh provides thin SSH/SFTP primitives used by the remote-provisioning
// layer: connecting with TOFU host-key pinning, running commands with captured
// exit codes, and file transfer.
package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Config describes how to connect to a remote host.
type Config struct {
	Host     string
	Port     int
	User     string
	AuthKind string // "agent" | "key_path"
	KeyRef   string // private key file path when AuthKind == "key_path"

	// PinnedHostKey is the previously accepted host key in authorized_keys form
	// ("ssh-ed25519 AAAA... "). Empty enables TOFU (accept + record on connect).
	PinnedHostKey string

	Timeout time.Duration
}

// Client wraps an SSH connection plus the host key that was seen/pinned.
type Client struct {
	ssh          *xssh.Client
	SeenHostKey  string // authorized_keys line observed this connection
	PinnedOnDial bool   // true if TOFU recorded a new key
}

// Dial connects to the host, verifying (or pinning, on TOFU) the host key.
func Dial(cfg Config) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 12 * time.Second
	}
	auth, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}

	c := &Client{}
	hkCb := func(_ string, _ net.Addr, key xssh.PublicKey) error {
		line := strings.TrimSpace(string(xssh.MarshalAuthorizedKey(key)))
		c.SeenHostKey = line
		if cfg.PinnedHostKey == "" {
			c.PinnedOnDial = true // TOFU: accept and let caller persist it
			return nil
		}
		if strings.TrimSpace(cfg.PinnedHostKey) != line {
			return fmt.Errorf("host key mismatch for %s: pinned=%s got=%s (possible MITM)",
				cfg.Host, safeFingerprint(cfg.PinnedHostKey), xssh.FingerprintSHA256(key))
		}
		return nil
	}

	clientCfg := &xssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hkCb,
		Timeout:         cfg.Timeout,
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	conn, err := xssh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	c.ssh = conn
	return c, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	if c.ssh != nil {
		return c.ssh.Close()
	}
	return nil
}

// Fingerprint returns the SHA256 fingerprint of the seen host key, if any.
func (c *Client) Fingerprint() string {
	return safeFingerprint(c.SeenHostKey)
}

// safeFingerprint returns the SHA256 fingerprint of an authorized_keys line,
// or "" if it cannot be parsed.
func safeFingerprint(line string) string {
	pk := mustParse(line)
	if pk == nil {
		return ""
	}
	return xssh.FingerprintSHA256(pk)
}

func authMethods(cfg Config) ([]xssh.AuthMethod, error) {
	switch cfg.AuthKind {
	case "", "agent":
		var methods []xssh.AuthMethod
		// Prefer a running ssh-agent when available.
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			if conn, err := net.Dial("unix", sock); err == nil {
				methods = append(methods, xssh.PublicKeysCallback(agent.NewClient(conn).Signers))
			}
		}
		// Fall back to standard identity files (matches ssh's default behavior),
		// so a host set up with a key file works without an agent.
		if signers := defaultIdentitySigners(); len(signers) > 0 {
			methods = append(methods, xssh.PublicKeys(signers...))
		}
		if len(methods) == 0 {
			return nil, errors.New("no ssh-agent and no usable ~/.ssh identity; use key_path auth")
		}
		return methods, nil
	case "key_path":
		path := expandHome(cfg.KeyRef)
		buf, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read key %s: %w", path, err)
		}
		signer, err := xssh.ParsePrivateKey(buf)
		if err != nil {
			return nil, fmt.Errorf("parse key %s: %w", path, err)
		}
		return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("unknown auth kind %q", cfg.AuthKind)
	}
}

// defaultIdentitySigners loads parseable, unencrypted private keys from the
// standard ~/.ssh locations.
func defaultIdentitySigners() []xssh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var signers []xssh.Signer
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		buf, err := os.ReadFile(home + "/.ssh/" + name)
		if err != nil {
			continue
		}
		if s, err := xssh.ParsePrivateKey(buf); err == nil {
			signers = append(signers, s)
		}
	}
	return signers
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
	}
	return p
}

// mustParse parses an authorized_keys line into a public key; returns nil on
// error (only used for fingerprint display where nil is tolerable).
func mustParse(line string) xssh.PublicKey {
	pk, _, _, _, err := xssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return nil
	}
	return pk
}
