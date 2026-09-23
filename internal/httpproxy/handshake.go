package httpproxy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Request is a parsed HTTP CONNECT request.
type Request struct {
	Host string
	Port int
}

// parseRequest reads and validates a single HTTP/1.1 CONNECT request line. Only
// CONNECT is supported, which routes through the SSH tunnel like the socks path.
func parseRequest(br *bufio.Reader) (*Request, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read request line: %w", err)
	}

	fields := strings.Fields(line)
	if len(fields) != 3 {
		return nil, &Error{Status: 400, msg: "malformed request line"}
	}
	if !strings.EqualFold(fields[0], "CONNECT") {
		return nil, ErrBadRequest
	}
	if !strings.EqualFold(fields[2], "HTTP/1.1") {
		return nil, ErrBadVersion
	}

	host, port, err := splitHostPort(fields[1])
	if err != nil {
		return nil, ErrBadAddress
	}
	return &Request{Host: host, Port: port}, nil
}

// discardHeaders reads and discards any remaining headers until the blank line
// that terminates the request, so the pump can start over a clean boundary.
func discardHeaders(br *bufio.Reader) error {
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		if line == "\r\n" || line == "\n" {
			return nil
		}
	}
}

// splitHostPort splits "host:port" into its components, allowing IPv6 brackets.
func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, errors.New("invalid port")
	}
	return host, port, nil
}

// sendReply writes an HTTP/1.1 status line and the empty header terminator.
func sendReply(w io.Writer, status int, message string) error {
	_, err := fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n\r\n", status, message)
	return err
}
