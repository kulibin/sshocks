package socks

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

func TestParseGreeting(t *testing.T) {
	// VER=0x05, NMETHODS=0x01, METHOD=0x00
	g, err := parseGreeting(bufio.NewReader(bytes.NewReader([]byte{0x05, 0x01, 0x00})))
	if err != nil {
		t.Fatalf("parseGreeting: %v", err)
	}
	if g.Version != 0x05 || g.NumMethods != 1 || len(g.Methods) != 1 {
		t.Fatalf("greeting = %+v", g)
	}
}

func TestParseGreetingBadVersion(t *testing.T) {
	_, err := parseGreeting(bufio.NewReader(bytes.NewReader([]byte{0x04, 0x01, 0x00})))
	if !errors.Is(err, ErrBadVersion) {
		t.Fatalf("expected ErrBadVersion, got %v", err)
	}
}

func TestParseGreetingNoMethods(t *testing.T) {
	_, err := parseGreeting(bufio.NewReader(bytes.NewReader([]byte{0x05, 0x00})))
	if !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("expected ErrUnknownMethod, got %v", err)
	}
}

func TestParseRequestIPv4(t *testing.T) {
	buf := bytes.NewBuffer([]byte{
		0x05, 0x01, 0x00, 0x01,
		10, 20, 30, 40,
		0x1F, 0x90, // 8080
	})
	req, err := parseRequest(bufio.NewReader(buf))
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Host != "10.20.30.40" || req.Port != 8080 {
		t.Fatalf("req = %+v", req)
	}
	if req.IsDomain {
		t.Fatalf("IsDomain should be false for IPv4")
	}
}

func TestParseRequestDomain(t *testing.T) {
	domain := "example.com"
	buf := bytes.NewBuffer([]byte{
		0x05, 0x01, 0x00, 0x03,
		byte(len(domain)),
	})
	buf.Write([]byte(domain))
	buf.Write([]byte{0x00, 0x16}) // port 22
	req, err := parseRequest(bufio.NewReader(buf))
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Host != domain || req.Port != 22 || !req.IsDomain {
		t.Fatalf("req = %+v", req)
	}
}

func TestParseRequestUnsupportedCommand(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	_, err := parseRequest(bufio.NewReader(buf))
	if err == nil {
		t.Fatal("expected error for BIND")
	}
	var e *Error
	if !errors.As(err, &e) || e.Code != StatusUnsupported {
		t.Fatalf("expected StatusUnsupported, got %v", err)
	}
}

func TestParseRequestBadAtyp(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x05, 0x01, 0x00, 0xFF})
	_, err := parseRequest(bufio.NewReader(buf))
	if !errors.Is(err, ErrBadAtyp) {
		t.Fatalf("expected ErrBadAtyp, got %v", err)
	}
}

func TestSendGreetingReply(t *testing.T) {
	var buf bytes.Buffer
	if err := sendGreetingReply(&buf); err != nil {
		t.Fatalf("sendGreetingReply: %v", err)
	}
	if got := buf.Bytes(); len(got) != 2 || got[0] != 0x05 || got[1] != 0x00 {
		t.Fatalf("greeting reply = % x, want 05 00", got)
	}
}

func TestSendConnectErr(t *testing.T) {
	var buf bytes.Buffer
	if err := sendConnectErr(&buf, StatusRefused); err != nil {
		t.Fatalf("sendConnectErr: %v", err)
	}
	got := buf.Bytes()
	if len(got) != 10 || got[0] != 0x05 || got[1] != byte(StatusRefused) {
		t.Fatalf("connect err = % x, want 10 bytes with reply code %02x", got, StatusRefused)
	}
}
