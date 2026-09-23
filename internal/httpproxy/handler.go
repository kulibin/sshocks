package httpproxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"

	"sshocks/internal/log"
)

// Dialer dials a target on the remote side of an SSH tunnel. The target host is
// resolved by the remote host, matching the socks package's contract.
type Dialer interface {
	DialTarget(ctx context.Context, host string, port int) (net.Conn, error)
}

// handleSession runs a single HTTP CONNECT session: parse the request, dial the
// target through the tunnel, and pump data both directions.
func handleSession(ctx context.Context, client net.Conn, dialer Dialer, logger log.Logger, sessionID int64) {
	defer client.Close()

	br := bufio.NewReader(client)

	req, err := parseRequest(br)
	if err != nil {
		if e := httpErr(err); e != nil {
			logger.Warnf("httpproxy: bad request (session %d): %v", sessionID, e)
			_ = sendReply(client, e.Status, e.msg)
		} else {
			logger.Warnf("httpproxy: request parse error (session %d): %v", sessionID, err)
		}
		return
	}

	if err := discardHeaders(br); err != nil {
		logger.Warnf("httpproxy: header parse error (session %d): %v", sessionID, err)
		return
	}

	logger.Debugf("httpproxy: CONNECT (session %d) -> %s:%d via tunnel", sessionID, req.Host, req.Port)

	remote, err := dialer.DialTarget(ctx, req.Host, req.Port)
	if err != nil {
		logger.Warnf("httpproxy: dial %s:%d failed (session %d): %v",
			req.Host, req.Port, sessionID, err)
		_ = sendReply(client, 502, "Bad Gateway")
		return
	}
	defer remote.Close()

	if err := sendReply(client, 200, "Connection Established"); err != nil {
		logger.Warnf("httpproxy: connect OK reply (session %d): %v", sessionID, err)
		return
	}

	logger.Infof("httpproxy: session %d established %s -> %s:%d via tunnel",
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
		logger.Warnf("httpproxy: session %d stopped by context (drain)", sessionID)
		<-errc
		<-errc
		return

	case <-errc:
		_ = a.Close()
		_ = b.Close()
		<-errc
	}
}

// httpErr extracts an *Error from err via errors.As, or returns nil.
func httpErr(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}
