package httpproxy

import "fmt"

// Error is an HTTP proxy protocol error that carries an HTTP status code a
// client should receive in the CONNECT reply.
type Error struct {
	Status int
	msg    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("httpproxy: %s (status %d)", e.msg, e.Status)
}

// Predefined errors.
var (
	ErrBadVersion   = &Error{Status: 400, msg: "unsupported HTTP version"}
	ErrBadRequest   = &Error{Status: 400, msg: "unsupported request"}
	ErrBadAddress   = &Error{Status: 400, msg: "malformed target host:port"}
	ErrUnauthorized = &Error{Status: 407, msg: "proxy authentication required"}
	ErrRefused      = &Error{Status: 502, msg: "connection refused or failed"}
)
