package socks

import "fmt"

// StatusCode is a SOCKS5 reply code per RFC 1928.
type StatusCode uint8

// Standard SOCKS5 reply status codes.
const (
	StatusOK          StatusCode = 0x00
	StatusGeneral     StatusCode = 0x01
	StatusNetworkDown StatusCode = 0x04
	StatusRefused     StatusCode = 0x05
	StatusUnsupported StatusCode = 0x08
	StatusBadRequest  StatusCode = 0x70
)

// Error is a SOCKS5 protocol error that carries a reply code a client should
// receive.
type Error struct {
	Code StatusCode
	msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("socks5: %s (code 0x%02x)", e.msg, e.Code)
}

// Predefined errors.
var (
	ErrBadVersion      = &Error{Code: StatusBadRequest, msg: "unsupported SOCKS5 version"}
	ErrUnknownMethod   = &Error{Code: StatusRefused, msg: "unknown auth method"}
	UnsupportedCommand = &Error{Code: StatusUnsupported, msg: "unsupported command"}
	ErrBadAtyp         = &Error{Code: StatusBadRequest, msg: "unsupported address type"}
)
