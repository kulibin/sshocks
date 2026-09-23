package httpproxy

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

func TestParseRequestConnect(t *testing.T) {
	line := "CONNECT example.com:443 HTTP/1.1\r\n\r\n"
	req, err := parseRequest(bufio.NewReader(bytes.NewReader([]byte(line))))
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Host != "example.com" || req.Port != 443 {
		t.Fatalf("req = %+v", req)
	}
}

func TestParseRequestNonConnect(t *testing.T) {
	line := "GET /index.html HTTP/1.1\r\nHost: example.com\r\n\r\n"
	_, err := parseRequest(bufio.NewReader(bytes.NewReader([]byte(line))))
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestParseRequestBadVersion(t *testing.T) {
	line := "CONNECT example.com:443 HTTP/1.0\r\n\r\n"
	_, err := parseRequest(bufio.NewReader(bytes.NewReader([]byte(line))))
	if !errors.Is(err, ErrBadVersion) {
		t.Fatalf("expected ErrBadVersion, got %v", err)
	}
}

func TestParseRequestBadAddress(t *testing.T) {
	line := "CONNECT example.com HTTP/1.1\r\n\r\n"
	_, err := parseRequest(bufio.NewReader(bytes.NewReader([]byte(line))))
	if !errors.Is(err, ErrBadAddress) {
		t.Fatalf("expected ErrBadAddress, got %v", err)
	}
}

func TestParseRequestIPv6(t *testing.T) {
	line := "CONNECT [::1]:8443 HTTP/1.1\r\n\r\n"
	req, err := parseRequest(bufio.NewReader(bytes.NewReader([]byte(line))))
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Host != "::1" || req.Port != 8443 {
		t.Fatalf("req = %+v", req)
	}
}

func TestDiscardHeaders(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader([]byte("Host: example.com\r\nUser-Agent: x\r\n\r\n")))
	if err := discardHeaders(r); err != nil {
		t.Fatalf("discardHeaders: %v", err)
	}
}

func TestSendReply(t *testing.T) {
	var buf bytes.Buffer
	if err := sendReply(&buf, 200, "Connection Established"); err != nil {
		t.Fatalf("sendReply: %v", err)
	}
	got := buf.String()
	if got != "HTTP/1.1 200 Connection Established\r\n\r\n" {
		t.Fatalf("reply = %q", got)
	}
}
