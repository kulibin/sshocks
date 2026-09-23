package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// Resolver resolves a target hostname to an IP on the local machine, bypassing a
// possibly broken local resolver by dialing the configured DNS servers directly.
// When no resolver is configured, hostnames are handed to the remote side of the
// SSH tunnel for resolution (the default behaviour).
type Resolver interface {
	// Resolve returns a target IP for host. A zero IP with a nil error means
	// "not resolved here" (the host is already an IP literal, or no server
	// applies) and the caller should fall back to remote resolution. A non-nil
	// error means resolution was attempted and failed.
	Resolve(ctx context.Context, host string) (string, error)
}

// dnsResolver resolves hostnames against an explicit list of DNS servers, bypassing
// the OS resolver. It dials each server's UDP/TCP 53 socket directly so a
// broken or censored local resolver does not affect the lookup.
type dnsResolver struct {
	r *net.Resolver
}

// NewDNSResolver builds a resolver for the given servers, or returns nil when the
// list is empty, signalling "no local resolution" so callers fall back to remote
// resolution.
func NewDNSResolver(servers []string, timeout time.Duration) (*dnsResolver, error) {
	if len(servers) == 0 {
		return nil, nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	parsed := make([]string, 0, len(servers))
	for _, s := range servers {
		host, port, err := net.SplitHostPort(s)
		if err != nil {
			host, port = s, "53"
		}
		if port == "" {
			port = "53"
		}
		parsed = append(parsed, net.JoinHostPort(host, port))
	}

	dialer := &net.Dialer{Timeout: timeout}
	// next spreads concurrent lookups across the configured servers.
	var next int32
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			// Force one of our own servers, bypassing whatever the local
			// resolver would otherwise select.
			addr := parsed[atomic.AddInt32(&next, 1)%int32(len(parsed))]
			return dialer.DialContext(ctx, network, addr)
		},
	}
	return &dnsResolver{r: r}, nil
}

// Resolve looks up host against the configured servers. If host is already an IP
// literal it is returned unchanged so the target is dialled directly. A lookup
// failure is returned as an error and does not fall back to remote resolution.
func (d *dnsResolver) Resolve(ctx context.Context, host string) (string, error) {
	if net.ParseIP(host) != nil {
		return host, nil
	}
	ips, err := d.r.LookupIPAddr(ctx, host)
	if err != nil {
		return "", fmt.Errorf("dns: resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("dns: no record for %q", host)
	}
	return ips[0].String(), nil
}

var _ Resolver = (*dnsResolver)(nil)
