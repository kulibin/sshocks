package tunnel

import (
	"context"
	"testing"
	"time"
)

func TestNewDNSEmptyReturnsNil(t *testing.T) {
	r, err := NewDNSResolver(nil, 0)
	if err != nil {
		t.Fatalf("NewDNSResolver(nil) error = %v, want nil", err)
	}
	if r != nil {
		t.Fatalf("NewDNSResolver(nil) = %v, want nil resolver", r)
	}
}

func TestNewDNSResolverBareIP(t *testing.T) {
	// A bare IP (no port) is accepted and defaults to port 53.
	r, err := NewDNSResolver([]string{"8.8.8.8"}, 0)
	if err != nil {
		t.Fatalf("NewDNSResolver error = %v, want nil", err)
	}
	if r == nil {
		t.Fatal("NewDNSResolver = nil, want resolver")
	}
}

func TestDNSResolverIPLiteralShortCircuit(t *testing.T) {
	r, err := NewDNSResolver([]string{"8.8.8.8"}, time.Second)
	if err != nil {
		t.Fatalf("NewDNSResolver error = %v", err)
	}
	ip, err := r.Resolve(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("Resolve(10.0.0.1) error = %v, want nil", err)
	}
	if ip != "10.0.0.1" {
		t.Errorf("Resolve(10.0.0.1) = %q, want %q (IP literal bypasses lookup)", ip, "10.0.0.1")
	}
}

func TestDNSResolverUnreachableServerErrors(t *testing.T) {
	// A dead server must surface an error, not fall back to remote resolution.
	r, err := NewDNSResolver([]string{"127.0.0.1:1"}, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDNSResolver error = %v", err)
	}
	_, err = r.Resolve(context.Background(), "example.com")
	if err == nil {
		t.Fatal("Resolve against a dead server = nil, want error")
	}
}
