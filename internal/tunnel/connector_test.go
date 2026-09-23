package tunnel

import (
	"errors"
	"testing"
	"time"
)

// selectorLog is a minimal logger for tests that captures nothing.
type selectorLog struct{}

func (selectorLog) Debugf(format string, args ...any) {}
func (selectorLog) Infof(format string, args ...any)  {}
func (selectorLog) Warnf(format string, args ...any)  {}
func (selectorLog) Errorf(format string, args ...any) {}

func TestSelectorPasswordPriority(t *testing.T) {
	cfg := Config{
		Password:      "secret",
		Key:           "/home/user/.ssh/id_ed25519",
		KeyPassphrase: "",
	}
	if got := cfg.Selector(); got != "password" {
		t.Errorf("Selector() = %q, want password (password takes priority)", got)
	}
}

func TestSelectorKeyNoPassword(t *testing.T) {
	cfg := Config{
		Key:           "/home/user/.ssh/id_ed25519",
		KeyPassphrase: "pass",
	}
	if got := cfg.Selector(); got != "key" {
		t.Errorf("Selector() = %q, want key (no password)", got)
	}
}

func TestSelectorNoAuth(t *testing.T) {
	cfg := Config{}
	if got := cfg.Selector(); got != "none" {
		t.Errorf("Selector() = %q, want none", got)
	}
}

func TestNewConfigDefaults(t *testing.T) {
	cfg := NewConfig("h", 0, "u", "p", "", "", 0)
	if cfg.Port != 22 {
		t.Errorf("Port = %d, want 22", cfg.Port)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestNextBackoffCaps(t *testing.T) {
	c := NewConnector(Config{}, selectorLog{})
	if got := c.nextBackoff(1 * time.Second); got != 2*time.Second {
		t.Errorf("nextBackoff(1s) = %v, want 2s", got)
	}
	if got := c.nextBackoff(30 * time.Second); got != 30*time.Second {
		t.Errorf("nextBackoff(30s) = %v, want 30s (cap)", got)
	}
	if got := c.nextBackoff(40 * time.Second); got != 30*time.Second {
		t.Errorf("nextBackoff(40s) = %v, want 30s (cap)", got)
	}
}

func TestClientErrNotConnected(t *testing.T) {
	c := NewConnector(Config{}, selectorLog{})
	_, err := c.Client()
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("Client() error = %v, want ErrNotConnected", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	c := NewConnector(Config{}, selectorLog{})
	if err := c.Close(); err != nil {
		t.Errorf("first Close() = %v, want nil", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil (idempotent)", err)
	}
}
