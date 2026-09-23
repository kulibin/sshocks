package socks

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"

	"sshocks/internal/log"
)

// Dialer dials a target on the remote side of an SSH tunnel. The target host is
// resolved by the remote host.
type Dialer interface {
	DialTarget(ctx context.Context, host string, port int) (net.Conn, error)
}

// handleSession runs a single SOCKS5 TCP session: parse the greeting, parse the
// request, dial the target through the tunnel, and pump data both directions.
func handleSession(ctx context.Context, client net.Conn, dialer Dialer, logger log.Logger, sessionID int64) {
	defer client.Close()

	br := bufio.NewReader(client)

	// Greeting.
	if _, err := parseGreeting(br); err != nil {
		if e := socksErr(err); e != nil {
			logger.Warnf("socks: bad greeting (session %d): %v", sessionID, e)
			_ = sendConnectErr(client, e.Code)
		} else {
			logger.Warnf("socks: greeting parse error (session %d): %v", sessionID, err)
		}
		return
	}

	if err := sendGreetingReply(client); err != nil {
		logger.Warnf("socks: greeting reply (session %d): %v", sessionID, err)
		return
	}

	// Request.
	req, err := parseRequest(br)
	if err != nil {
		if e := socksErr(err); e != nil {
			logger.Warnf("socks: bad request (session %d): %v", sessionID, e)
			_ = sendConnectErr(client, e.Code)
		} else {
			logger.Warnf("socks: request parse error (session %d): %v", sessionID, err)
		}
		return
	}

	logger.Debugf("socks: CONNECT (session %d) -> %s:%d via tunnel", sessionID, req.Host, req.Port)

	remote, err := dialer.DialTarget(ctx, req.Host, req.Port)
	if err != nil {
		logger.Warnf("socks: dial %s:%d failed (session %d): %v",
			req.Host, req.Port, sessionID, err)
		_ = sendConnectErr(client, StatusRefused)
		return
	}
	defer remote.Close()

	if err := sendConnectOK(client); err != nil {
		logger.Warnf("socks: connect OK reply (session %d): %v", sessionID, err)
		return
	}

	logger.Infof("socks: session %d established %s -> %s:%d via tunnel",
		sessionID, client.RemoteAddr(), req.Host, req.Port)

	pump(ctx, client, remote, logger, sessionID)
}

// pump copies bytes between two connections until either closes or ctx is
// cancelled, then closes both.
func pump(ctx context.Context, a, b net.Conn, logger log.Logger, sessionID int64) {
	errc := make(chan error, 2)

	go func() {
		_, err := io.Copy(b, a)
		errc <- err
		if c, ok := b.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
	}()
	go func() {
		_, err := io.Copy(a, b)
		errc <- err
		if c, ok := a.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
	}()

	select {
	case <-ctx.Done():
		_ = a.Close()
		_ = b.Close()
		logger.Warnf("socks: session %d stopped by context (drain)", sessionID)
		// drain the two pump goroutines
		<-errc
		<-errc
		return

	case <-errc:
		// First direction finished; close both ends and drain the rest.
		_ = a.Close()
		_ = b.Close()
		<-errc
	}
}

// socksErr extracts a *Error from err via errors.As, or returns nil.
func socksErr(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}
