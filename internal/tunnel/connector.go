package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// backoff constants for the reconnect loop.
const (
	// backoffStart is the first reconnect delay.
	backoffStart = 1 * time.Second
	// backoffCap caps the exponential backoff.
	backoffCap = 30 * time.Second
	// keepAliveInterval is how often the watcher probes the connection.
	keepAliveInterval = 1 * time.Second
)

// logger is the minimal logging surface the tunnel needs.
type logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// ErrNotConnected is returned by Client and DialTarget when no SSH session is
// established.
var ErrNotConnected = errors.New("ssh: not connected")

// Config describes the SSH tunnel target and credentials.
type Config struct {
	Host          string
	Port          int
	User          string
	Password      string
	Key           string
	KeyPassphrase string
	Timeout       time.Duration
}

// NewConfig builds a tunnel.Config, applying defaults for port and timeout.
func NewConfig(host string, port int, user, password, key, keyPassphrase string, timeout time.Duration) Config {
	if port == 0 {
		port = 22
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return Config{
		Host:          host,
		Port:          port,
		User:          user,
		Password:      password,
		Key:           key,
		KeyPassphrase: keyPassphrase,
		Timeout:       timeout,
	}
}

// Selector reports which authentication method is used first. Password takes
// priority over key (intentional per project requirements).
func (c Config) Selector() string {
	if c.Password != "" {
		return "password"
	}
	if c.Key != "" {
		return "key"
	}
	return "none"
}

// Connector maintains a single SSH connection to the remote host and
// automatically reconnects on loss with exponential backoff.
type Connector struct {
	cfg Config
	log logger

	mu     sync.RWMutex
	client *ssh.Client
	lostCh chan struct{}
}

// NewConnector builds a Connector. It does not dial; call Run.
func NewConnector(cfg Config, log logger) *Connector {
	return &Connector{
		cfg:    cfg,
		log:    log,
		lostCh: make(chan struct{}),
	}
}

// Run dials the SSH server and keeps it healthy, reconnecting on loss until ctx
// is cancelled. Returns after ctx cancellation.
func (c *Connector) Run(ctx context.Context) error {
	backoff := backoffStart
	for {
		if err := c.connect(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			c.log.Errorf("ssh: connect failed after %s: %v", backoff, err)
			if c.sleep(ctx, backoff) {
				break // ctx cancelled
			}
			backoff = c.nextBackoff(backoff)
			continue
		}
		backoff = backoffStart
		c.log.Debugf("ssh: waiting for connection loss...")
		// Block until the watcher reports a loss or ctx is cancelled.
		select {
		case <-c.lostCh:
			c.log.Warnf("ssh: connection lost, reconnecting")
		case <-ctx.Done():
			break
		}
	}
	c.log.Infof("ssh: connector stopped")
	return nil
}

// connect attempts a single connection, returns an error on failure.
func (c *Connector) connect(ctx context.Context) error {
	dialer := net.Dialer{Timeout: c.cfg.Timeout}

	ctxD, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	c.log.Infof("ssh: dialing %s", addr)

	nconn, err := dialer.DialContext(ctxD, "tcp", addr)
	if err != nil {
		return err
	}

	sshCfg := &ssh.ClientConfig{
		User:            c.cfg.User,
		Auth:            c.authMethods(),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.cfg.Timeout,
	}
	c.log.Debugf("ssh: using auth method %s", c.cfg.Selector())

	conn, chans, reqs, err := ssh.NewClientConn(nconn, addr, sshCfg)
	if err != nil {
		_ = nconn.Close()
		return fmt.Errorf("ssh: handshake: %w", err)
	}
	client := ssh.NewClient(conn, chans, reqs)
	c.setClient(client)
	c.log.Infof("ssh: connected to %s/%d as %s (auth=%s)",
		c.cfg.Host, c.cfg.Port, c.cfg.User, c.cfg.Selector())
	return nil
}

// authMethods builds the SSH auth methods, password first then public key.
func (c *Connector) authMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	if c.cfg.Password != "" {
		methods = append(methods, ssh.Password(c.cfg.Password))
	}
	if c.cfg.Key != "" {
		if am, err := c.pubkeyAuth(); err == nil {
			methods = append(methods, am)
		} else {
			c.log.Warnf("ssh: public key auth unavailable: %v", err)
		}
	}
	return methods
}

// pubkeyAuth loads the private key (optionally passphrase-protected) and wraps
// it as an ssh.AuthMethod.
func (c *Connector) pubkeyAuth() (ssh.AuthMethod, error) {
	keyPath := c.cfg.Key
	if home := userHome(); home != "" && len(keyPath) >= 2 && keyPath[:2] == "~/" {
		keyPath = filepath.Join(home, keyPath[2:])
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key %q: %w", keyPath, err)
	}

	var signer ssh.Signer
	if c.cfg.KeyPassphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(c.cfg.KeyPassphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyBytes)
	}
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	return ssh.PublicKeys(signer), nil
}

// nextBackoff doubles the backoff up to backoffCap.
func (c *Connector) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > backoffCap {
		next = backoffCap
	}
	return next
}

// setClient stores the active client and starts the watcher for this client.
func (c *Connector) setClient(client *ssh.Client) {
	c.mu.Lock()
	c.client = client
	// Fresh lostCh per connection so a stale loss does not retrigger.
	c.lostCh = make(chan struct{})
	lost := c.lostCh
	c.mu.Unlock()

	go c.watch(client, lost)
}

// watch probes the connection on an interval and closes lost when it fails.
func (c *Connector) watch(client *ssh.Client, lost chan struct{}) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// wantReply=false so the probe never blocks on an ignored request;
			// a non-nil error means the transport is down.
			if _, _, err := client.SendRequest("keepalive@sshocks.local", false, nil); err != nil {
				c.signalLost(lost)
				return
			}
		}
	}
}

// signalLost closes lost exactly once.
func (c *Connector) signalLost(lost chan struct{}) {
	select {
	case <-lost:
	default:
		close(lost)
	}
}

// sleep blocks for d or until ctx is cancelled. Returns true if ctx was
// cancelled during the sleep (caller should stop).
func (c *Connector) sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

// Client returns the active SSH client or ErrNotConnected if none is set.
func (c *Connector) Client() (*ssh.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.client == nil {
		return nil, ErrNotConnected
	}
	return c.client, nil
}

// Close closes the SSH client and releases resources. Idempotent.
func (c *Connector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return nil
	}
	err := c.client.Close()
	c.client = nil
	return err
}

// userHome returns the current user's home directory, or "".
func userHome() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return ""
}
