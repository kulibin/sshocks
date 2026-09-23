package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
)

// DialTarget dials host:port on the remote side of the SSH tunnel. When a
// resolver is configured, a hostname is resolved locally first (bypassing a
// broken local resolver) and the resulting IP is forwarded; otherwise the host
// is resolved by the remote host (the point of a tunnel).
// Returns ErrNotConnected if no session is active.
func (c *Connector) DialTarget(ctx context.Context, host string, port int) (net.Conn, error) {
	client, err := c.Client()
	if err != nil {
		return nil, err
	}
	if host == "" {
		return nil, errors.New("ssh dial: empty target host")
	}

	c.mu.RLock()
	resolver := c.resolver
	c.mu.RUnlock()
	if resolver != nil {
		ip, err := resolver.Resolve(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("ssh dial target %s: %w", host, err)
		}
		if ip != "" {
			host = ip
		}
	}

	target := net.JoinHostPort(host, strconv.Itoa(port))
	c.log.Debugf("ssh: dialing target %s through tunnel", target)

	// Use the client's Dial which performs a remote forward over the existing
	// SSH session.
	ch, err := client.Dial("tcp", target)
	if err != nil {
		return nil, fmt.Errorf("ssh dial target %s: %w", target, err)
	}
	select {
	case <-ctx.Done():
		_ = ch.Close()
		return nil, ctx.Err()
	default:
		return ch, nil
	}
}
