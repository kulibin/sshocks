package socks

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// Greeting represents the client's initial method-selection request. With no
// authentication (a local tool), we always select method 0x00 (NO-AUTH).
type Greeting struct {
	Version    byte
	NumMethods byte
	Methods    []byte
}

// parseGreeting reads and validates the SOCKS5 greeting. RFC 1928 §3.
func parseGreeting(br *bufio.Reader) (*Greeting, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return nil, fmt.Errorf("read greeting header: %w", err)
	}
	if hdr[0] != 0x05 {
		return nil, ErrBadVersion
	}
	n := hdr[1]
	if n == 0 {
		return nil, ErrUnknownMethod
	}
	methods := make([]byte, n)
	if _, err := io.ReadFull(br, methods); err != nil {
		return nil, fmt.Errorf("read greeting methods: %w", err)
	}
	return &Greeting{Version: 0x05, NumMethods: n, Methods: methods}, nil
}

// sendGreetingReply writes the SERVER-CHOOSING reply. With no auth we always
// choose 0x00 (NO-AUTH).
func sendGreetingReply(w io.Writer) error {
	_, err := w.Write([]byte{0x05, 0x00})
	return err
}

// Command is a SOCKS5 request command.
type Command byte

// CONNECT is the only supported command in v1.
const (
	CmdConnect  Command = 0x01
	CmdBind     Command = 0x02
	CmdUdpAssoc Command = 0x03
)

// Atyp is an address type byte.
const (
	// AtypIPv4 is the IPv4 address type.
	AtypIPv4 byte = 0x01
	// AtypDomain is the domain-name address type.
	AtypDomain byte = 0x03
)

// Request is a parsed SOCKS5 request.
type Request struct {
	Version  byte
	Command  Command
	Host     string
	Port     int
	IsDomain bool
}

// parseRequest reads and validates a SOCKS5 CONNECT request. RFC 1928 §4.
func parseRequest(br *bufio.Reader) (*Request, error) {
	hdr := make([]byte, 3)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return nil, fmt.Errorf("read request header: %w", err)
	}
	if hdr[0] != 0x05 {
		return nil, ErrBadVersion
	}
	cmd := Command(hdr[1])
	if cmd != CmdConnect {
		return nil, &Error{Code: StatusUnsupported, msg: fmt.Sprintf("unsupported command 0x%02x", cmd)}
	}
	if reserved := hdr[2]; reserved != 0x00 {
		return nil, &Error{Code: StatusBadRequest, msg: "non-zero reserved byte"}
	}

	req := &Request{Version: 0x05, Command: cmd}
	if err := req.readAddress(br); err != nil {
		return nil, err
	}
	return req, nil
}

// readAddress reads the address portion (ATYP + data + port) and populates
// host/port on the request.
func (r *Request) readAddress(br *bufio.Reader) error {
	var atypBuf [1]byte
	if _, err := io.ReadFull(br, atypBuf[:]); err != nil {
		return fmt.Errorf("read atyp: %w", err)
	}
	atyp := atypBuf[0]

	switch atyp {
	case AtypIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(br, ip); err != nil {
			return fmt.Errorf("read IPv4 addr: %w", err)
		}
		r.Host = ipString(ip)
		r.IsDomain = false
		port, err := readPort(br)
		if err != nil {
			return err
		}
		r.Port = port
		return nil

	case AtypDomain:
		var lenBuf [1]byte
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			return fmt.Errorf("read domain length: %w", err)
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(br, domain); err != nil {
			return fmt.Errorf("read domain: %w", err)
		}
		port, err := readPort(br)
		if err != nil {
			return err
		}
		r.Host = string(domain)
		r.Port = port
		r.IsDomain = true
		return nil

	default:
		return ErrBadAtyp
	}
}

// readPort reads a 2-byte big-endian port.
func readPort(br *bufio.Reader) (int, error) {
	var b [2]byte
	if _, err := io.ReadFull(br, b[:]); err != nil {
		return 0, fmt.Errorf("read port: %w", err)
	}
	return int(binary.BigEndian.Uint16(b[:])), nil
}

// sendConnectOK writes a success reply. The BND.ADDR is a placeholder since a
// local tunnel client typically ignores it.
func sendConnectOK(w io.Writer) error {
	reply := []byte{0x05, 0x00, 0x00, AtypIPv4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := w.Write(reply)
	return err
}

// sendConnectErr writes a failure reply with the given status code.
func sendConnectErr(w io.Writer, code StatusCode) error {
	reply := []byte{0x05, byte(code), 0x00, AtypIPv4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := w.Write(reply)
	return err
}

// ipString renders a 4-byte IPv4 address as dotted decimal.
func ipString(ip []byte) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}
